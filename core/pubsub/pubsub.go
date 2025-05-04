package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	log "github.com/sirupsen/logrus"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type topicPrefix string

func (t topicPrefix) isIn(s string) bool {
	return strings.HasPrefix(s, string(t))
}

const (
	// full names
	pubSubDiscoveryTopic     = "peer-discovery"
	leaderAnnouncementsTopic = "leader-announcements"

	// prefixes
	userUpdateTopicPrefix topicPrefix = "user-update"
)

type PubsubServerNodeConnector interface {
	Node() warpnet.P2PNode
	Connect(warpnet.PeerAddrInfo) error
	NodeInfo() warpnet.NodeInfo
}

type PubsubClientNodeStreamer interface {
	ClientStream(nodeId string, path string, data any) (_ []byte, err error)
}

type PubsubFollowingStorer interface {
	GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
}

type warpPubSub struct {
	ctx        context.Context
	pubsub     *pubsub.PubSub
	serverNode PubsubServerNodeConnector
	clientNode PubsubClientNodeStreamer
	followRepo PubsubFollowingStorer

	ownerId string

	mx                *sync.RWMutex
	subs              []*pubsub.Subscription
	relayCancelFuncs  map[string]pubsub.RelayCancelFunc
	topics            map[string]*pubsub.Topic
	discoveryHandlers []discovery.DiscoveryHandler

	isRunning *atomic.Bool
}

func NewPubSub(
	ctx context.Context,
	followRepo PubsubFollowingStorer,
	ownerId string,
	discoveryHandlers ...discovery.DiscoveryHandler,
) *warpPubSub {

	g := &warpPubSub{
		ctx:               ctx,
		pubsub:            nil,
		serverNode:        nil,
		clientNode:        nil,
		discoveryHandlers: discoveryHandlers,
		mx:                new(sync.RWMutex),
		subs:              []*pubsub.Subscription{},
		topics:            map[string]*pubsub.Topic{},
		relayCancelFuncs:  map[string]pubsub.RelayCancelFunc{},
		followRepo:        followRepo,
		ownerId:           ownerId,
		isRunning:         new(atomic.Bool),
	}

	return g
}

func NewPubSubBootstrap(
	ctx context.Context,
	discoveryHandlers ...discovery.DiscoveryHandler,
) *warpPubSub {
	return NewPubSub(ctx, nil, "", discoveryHandlers...)
}

func (g *warpPubSub) Run(
	serverNode PubsubServerNodeConnector, clientNode PubsubClientNodeStreamer,
) {
	if g.isRunning.Load() {
		return
	}
	defer log.Infoln("pubsub: stopped")

	g.clientNode = clientNode
	g.serverNode = serverNode

	if err := g.runPubSub(serverNode); err != nil {
		log.Errorf("pubsub: failed to run: %v", err)
		return
	}
	if err := g.subscribeFollowees(); err != nil {
		log.Errorf("pubsub: presubscribe: %v", err)
		return
	}
	if err := g.runListener(); err != nil {
		log.Errorf("pubsub: listener: %v", err)
		return
	}
}

func (g *warpPubSub) runListener() error {
	if g == nil {
		return errors.New("pubsub: service not initialized properly")
	}
	for {
		if !g.isRunning.Load() {
			return nil
		}

		g.mx.RLock()
		subscriptions := g.subs
		g.mx.RUnlock()

		for _, sub := range subscriptions {
			if err := g.ctx.Err(); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			msg, err := sub.Next(ctx)
			cancel()
			if errors.Is(err, pubsub.ErrSubscriptionCancelled) {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				continue
			}
			if err != nil {
				return err
			}
			if msg.Topic == nil {
				continue
			}

			// full topic names match
			switch strings.TrimSpace(*msg.Topic) {
			case pubSubDiscoveryTopic:
				g.handlePubSubDiscovery(msg)
				continue
			case leaderAnnouncementsTopic:
				log.WithField("topic", *msg.Topic).Info("pubsub: leader announcement:", string(msg.Data)) // TODO
				continue
			default:
			}

			// topic prefixes match
			switch {
			case userUpdateTopicPrefix.isIn(*msg.Topic):
				if err := g.handleUserUpdate(msg); err != nil {
					log.Infof("pubsub: user update error: %v", err)
				}
				continue
			default:
				log.Warnf("pubsub: unknown topic: %s, message: %s", *msg.Topic, string(msg.Data))
			}
		}
	}
}

func (g *warpPubSub) runPubSub(n PubsubServerNodeConnector) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pubsub: recovered from panic: %v", r)
		}
	}()
	if g == nil {
		return errors.New("pubsub: service not initialized properly")
	}

	g.pubsub, err = pubsub.NewGossipSub(g.ctx, n.Node())
	if err != nil {
		return err
	}
	g.isRunning.Store(true)

	if err := g.subscribe(pubSubDiscoveryTopic); err != nil {
		return err
	}
	if err := g.subscribe(leaderAnnouncementsTopic); err != nil {
		return err
	}

	go g.runPeerInfoPublishing()

	log.Infoln("pubsub: started")

	return nil
}

func (g *warpPubSub) subscribeFollowees() error {
	if g == nil {
		return errors.New("pubsub: service not initialized properly")
	}
	if g.ownerId == "" {
		return nil
	}
	if g.followRepo == nil {
		return nil
	}

	var (
		nextCursor string
		limit      = uint64(20)
	)
	for {
		followees, cur, err := g.followRepo.GetFollowees(g.ownerId, &limit, &nextCursor)
		if err != nil {
			return err
		}
		for _, f := range followees {
			if err := g.SubscribeUserUpdate(f.Followee); err != nil {
				return err
			}

		}
		if len(followees) < int(limit) {
			break
		}
		nextCursor = cur
	}

	log.Infoln("pubsub: followees presubscribed")
	return nil
}

func (g *warpPubSub) subscribe(topics ...string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	for _, topicName := range topics {
		if topicName == "" {
			return errors.New("pubsub: topic name is empty")
		}

		g.mx.RLock()
		topic, ok := g.topics[topicName]
		g.mx.RUnlock()
		if !ok {
			topic, err = g.pubsub.Join(topicName)
			if err != nil {
				return err
			}
		}

		g.mx.Lock()
		g.topics[topicName] = topic
		g.mx.Unlock()

		relayCancel, err := topic.Relay()
		if err != nil {
			return err
		}

		sub, err := topic.Subscribe()
		if err != nil {
			return err
		}

		log.Infof("pubsub: subscribed to topic: %s", topicName)

		g.mx.Lock()
		g.relayCancelFuncs[topicName] = relayCancel
		g.subs = append(g.subs, sub)
		g.mx.Unlock()
	}

	return nil
}

func (g *warpPubSub) GetSubscribers() peer.IDSlice {
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, g.ownerId)
	g.mx.RLock()
	topic, ok := g.topics[topicName]
	g.mx.RUnlock()
	if !ok {
		return nil
	}

	return topic.ListPeers()
}

func (g *warpPubSub) unsubscribe(topics ...string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}

	for _, topicName := range topics {
		g.mx.RLock()
		topic, ok := g.topics[topicName]
		g.mx.RUnlock()
		if !ok {
			return nil
		}

		g.mx.Lock()
		for i, s := range g.subs {
			if s.Topic() == topicName {
				s.Cancel()
				slices.Delete(g.subs, i, i+1)
				break
			}
		}

		if err = topic.Close(); err != nil {
			return err
		}
		delete(g.topics, topicName)

		if _, ok := g.relayCancelFuncs[topicName]; ok {
			g.relayCancelFuncs[topicName]()
		}
		delete(g.relayCancelFuncs, topicName)
		g.mx.Unlock()
	}

	return err
}

func (g *warpPubSub) publish(msg event.Message, topics ...string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}

	for _, topicName := range topics {
		g.mx.RLock()
		topic, ok := g.topics[topicName]
		g.mx.RUnlock()
		if !ok {
			topic, err = g.pubsub.Join(topicName)
			if err != nil {
				return err
			}
		}

		g.mx.Lock()
		g.topics[topicName] = topic
		g.mx.Unlock()

		data, err := json.Marshal(msg)
		if err != nil {
			log.Errorf("pubsub: failed to marshal owner update message: %v", err)
			return err
		}
		err = topic.Publish(g.ctx, data)
		if err != nil && !errors.Is(err, pubsub.ErrTopicClosed) {
			log.Errorf("pubsub: failed to publish owner update message: %v", err)
			return err
		}
	}

	return nil
}

// SubscribeUserUpdate - follow someone
func (g *warpPubSub) SubscribeUserUpdate(userId string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	if g.ownerId == userId {
		return errors.New("pubsub: can't subscribe to own user")
	}

	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, userId)
	return g.subscribe(topicName)
}

// UnsubscribeUserUpdate - unfollow someone
func (g *warpPubSub) UnsubscribeUserUpdate(userId string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, userId)
	return g.unsubscribe(topicName)
}

func (g *warpPubSub) handleUserUpdate(msg *pubsub.Message) error {
	var simulatedMessage event.Message
	if err := json.Unmarshal(msg.Data, &simulatedMessage); err != nil {
		log.Errorf("pubsub: failed to decode user update message: %v %s", err, msg.Data)
		return err
	}
	if simulatedMessage.NodeId == g.serverNode.NodeInfo().ID.String() {
		return nil
	}

	if simulatedMessage.Path == "" {
		log.Errorln("pubsub: user update message has no path", simulatedMessage.Path)
		return fmt.Errorf("pubsub: user update message has no path: %s", string(msg.Data))
	}
	if simulatedMessage.Body == nil {
		return nil
	}
	if stream.WarpRoute(simulatedMessage.Path).IsGet() { // only store data
		return nil
	}

	if g.clientNode == nil {
		return nil
	}

	log.Infof("pubsub: new user update: %s", *simulatedMessage.Body)

	_, err := g.clientNode.ClientStream( // send it to self
		g.serverNode.NodeInfo().ID.String(),
		simulatedMessage.Path,
		*simulatedMessage.Body,
	)
	return err
}

func (g *warpPubSub) handlePubSubDiscovery(msg *pubsub.Message) {
	var discoveryMsg warpnet.WarpAddrInfo
	if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
		log.Errorf("pubsub: discovery: failed to decode discovery message: %v %s", err, msg.Data)
		return
	}
	if discoveryMsg.ID == "" {
		log.Errorf("pubsub: discovery: message has no ID: %s", string(msg.Data))
		return
	}
	if discoveryMsg.ID == g.serverNode.NodeInfo().ID {
		return
	}

	peerInfo := warpnet.PeerAddrInfo{
		ID:    discoveryMsg.ID,
		Addrs: make([]warpnet.WarpAddress, 0, len(discoveryMsg.Addrs)),
	}

	for _, addr := range discoveryMsg.Addrs {
		ma, _ := warpnet.NewMultiaddr(addr)
		peerInfo.Addrs = append(peerInfo.Addrs, ma)
	}

	if g.discoveryHandlers != nil {
		for _, handler := range g.discoveryHandlers {
			handler(peerInfo) // add new user
		}
	}
}

// PublishOwnerUpdate - publish for followers
func (g *warpPubSub) PublishOwnerUpdate(ownerId string, msg event.Message) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, ownerId)

	return g.publish(msg, topicName)
}

func (g *warpPubSub) runPeerInfoPublishing() {
	g.mx.RLock()
	discTopic, ok := g.topics[pubSubDiscoveryTopic]
	g.mx.RUnlock()
	if !ok {
		log.Fatalf("pubsub: discovery topic not found: %s", pubSubDiscoveryTopic)
	}
	defer func() {
		_ = discTopic.Close()
	}()

	duration := time.Minute
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	log.Infoln("pubsub: publisher started")
	defer log.Infoln("pubsub: publisher stopped")

	if err := g.publishPeerInfo(discTopic); err != nil { // initial publishing
		log.Errorf("pubsub: failed to publish peer info: %v", err)
	}

	for {
		if !g.isRunning.Load() {
			return
		}

		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if err := g.publishPeerInfo(discTopic); err != nil {
				log.Errorf("pubsub: failed to publish peer info: %v", err)
				continue
			}
			ticker.Reset(duration * 2) // exponential prolonging
		}
	}
}

func (g *warpPubSub) publishPeerInfo(topic *pubsub.Topic) error {
	info := g.serverNode.NodeInfo()

	msg := warpnet.WarpAddrInfo{
		ID:    info.ID,
		Addrs: info.Addresses,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal peer info message: %v", err)
	}
	err = topic.Publish(g.ctx, data)
	if err != nil && !errors.Is(err, pubsub.ErrTopicClosed) {
		return err
	}
	return nil
}

func (g *warpPubSub) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	if !g.isRunning.Load() {
		return
	}

	g.mx.RLock()
	defer g.mx.RUnlock()

	for t, relayCancel := range g.relayCancelFuncs {
		relayCancel()
		delete(g.relayCancelFuncs, t)
	}

	for i, sub := range g.subs {
		sub.Cancel()
		g.subs[i] = nil
	}

	for t, topic := range g.topics {
		_ = topic.Close()
		delete(g.topics, t)
	}

	g.isRunning.Store(false)
	g.pubsub = nil
	return
}
