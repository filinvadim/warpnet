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
	log "github.com/sirupsen/logrus"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pubSubDiscoveryTopic     = "peer-discovery"
	leaderAnnouncementsTopic = "leader-announcements"
)

const (
	userUpdateTopicPrefix = "user-update-%s"
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
	ctx               context.Context
	pubsub            *pubsub.PubSub
	serverNode        PubsubServerNodeConnector
	clientNode        PubsubClientNodeStreamer
	discoveryHandlers []discovery.DiscoveryHandler

	mx         *sync.RWMutex
	subs       []*pubsub.Subscription
	topics     map[string]*pubsub.Topic
	ownerId    string
	followRepo PubsubFollowingStorer

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

		followRepo: followRepo,
		ownerId:    ownerId,
		isRunning:  new(atomic.Bool),
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

	if err := g.run(serverNode); err != nil {
		log.Fatalf("pubsub: failed to run: %s", err)
	}

	if err := g.preSubscribe(); err != nil {
		log.Fatalf("pubsub: failed to presubscribe: %s", err)
	}
	for {
		if !g.isRunning.Load() {
			return
		}

		g.mx.RLock()
		subscriptions := g.subs
		g.mx.RUnlock()

		for _, sub := range subscriptions {
			if g.ctx.Err() != nil {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			msg, err := sub.Next(ctx)
			if errors.Is(err, context.DeadlineExceeded) {
				cancel()
				continue
			}
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Infof("pubsub: subscription: %v", err)
				return
			}

			if msg.Topic == nil {
				continue
			}

			switch *msg.Topic {
			case pubSubDiscoveryTopic:
				g.handlePubSubDiscovery(msg)
			default:
				if strings.HasPrefix(*msg.Topic, userUpdateTopicPrefix) {
					if err := g.handleUserUpdate(msg); err != nil {
						log.Infof("pubsub: user update error: %v", err)
					}
				}
			}
		}
	}
}

func (g *warpPubSub) run(n PubsubServerNodeConnector) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("pubsub: recovered from panic: %v", r)
			err = errors.New("pubsub: recovered from panic")
		}
	}()
	if g == nil {
		panic("pubsub: service not initialized properly")
	}

	g.pubsub, err = pubsub.NewGossipSub(g.ctx, n.Node())
	if err != nil {
		return err
	}
	g.isRunning.Store(true)
	log.Infoln("pubsub: started")

	if err := g.GenericSubscribe(pubSubDiscoveryTopic); err != nil {
		return err
	}
	if err := g.GenericSubscribe(leaderAnnouncementsTopic); err != nil {
		return err
	}

	g.mx.RLock()
	discTopic := g.topics[pubSubDiscoveryTopic]
	g.mx.RUnlock()

	go g.publishPeerInfo(discTopic)
	return nil
}

func (g *warpPubSub) preSubscribe() error {
	var (
		nextCursor string
		limit      = uint64(20)
	)
	if g.ownerId == "" {
		return nil
	}
	if g.followRepo == nil {
		return nil
	}

	for {
		followees, cur, _ := g.followRepo.GetFollowees(g.ownerId, &limit, &nextCursor)
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

	for _, sub := range g.subs {
		sub.Cancel()
	}

	for i, topic := range g.topics {
		_ = topic.Close()
		g.topics[i] = nil
	}
	
	g.isRunning.Store(false)
	g.pubsub = nil
	return
}

// PublishOwnerUpdate - publish for followers
func (g *warpPubSub) PublishOwnerUpdate(ownerId string, msg event.Message) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	topicName := fmt.Sprintf(userUpdateTopicPrefix, ownerId)
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
	if err != nil {
		log.Errorf("pubsub: failed to publish owner update message: %v", err)
		return err
	}
	return
}

func (g *warpPubSub) GenericSubscribe(topicName string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
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

	if _, err := topic.Relay(); err != nil {
		return err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}
	log.Infof("pubsub: subscribed to topic: %s", topicName)
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()
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

	topicName := fmt.Sprintf(userUpdateTopicPrefix, userId)
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

	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}
	log.Infof("pubsub: subscribed to user update topic: %s", topicName)
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()
	return nil
}

// UnsubscribeUserUpdate - unfollow someone
func (g *warpPubSub) UnsubscribeUserUpdate(userId string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub: service not initialized")
	}
	topicName := fmt.Sprintf(userUpdateTopicPrefix, userId)
	g.mx.RLock()
	topic, ok := g.topics[topicName]
	g.mx.RUnlock()
	if !ok {
		return nil
	}

	err = topic.Close()

	g.mx.Lock()
	for i, s := range g.subs {
		if s.Topic() == topicName {
			s.Cancel()
			slices.Delete(g.subs, i, i+1)
			break
		}
	}
	delete(g.topics, topicName)
	g.mx.Unlock()

	return err
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

	_, err := g.clientNode.ClientStream( // send to self
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

func (g *warpPubSub) publishPeerInfo(discTopic *pubsub.Topic) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	log.Infoln("pubsub: publisher started")
	defer log.Infoln("pubsub: publisher stopped")

	for {
		if !g.isRunning.Load() {

			return
		}
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			addrs := make([]string, 0, len(g.serverNode.NodeInfo().Addrs))
			for _, addr := range g.serverNode.NodeInfo().Addrs {
				if addr == "" {
					continue
				}
				addrs = append(addrs, addr)
			}

			msg := warpnet.WarpAddrInfo{
				ID:    g.serverNode.NodeInfo().ID,
				Addrs: addrs,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Errorf("pubsub: failed to marshal peer info message: %v", err)
				return
			}
			err = discTopic.Publish(g.ctx, data)
			if err != nil {
				log.Errorf("pubsub: failed to publish peer info message: %v", err)
			}
		}
	}
}
