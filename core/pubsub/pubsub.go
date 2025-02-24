package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
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
	userUpdateTopicPrefix = "user-update-%s-%s"
)

type PubsubServerNodeConnector interface {
	Node() warpnet.P2PNode
	Connect(warpnet.PeerAddrInfo) error
	ID() warpnet.WarpPeerID
	Addrs() []string
}

type PubsubClientNodeStreamer interface {
	ClientStream(nodeId string, path string, data any) (_ []byte, err error)
}

type PubsubFollowingStorer interface {
	GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
}

type PubsubAuthStorer interface {
	GetOwner() domain.Owner
}

type Gossip struct {
	ctx              context.Context
	pubsub           *pubsub.PubSub
	serverNode       PubsubServerNodeConnector
	clientNode       PubsubClientNodeStreamer
	discoveryHandler discovery.DiscoveryHandler
	version          string
	owner            domain.Owner

	mx     *sync.RWMutex
	subs   []*pubsub.Subscription
	topics map[string]*pubsub.Topic

	isRunning *atomic.Bool
}

func NewPubSub(ctx context.Context, discoveryHandler discovery.DiscoveryHandler) *Gossip {
	version := fmt.Sprintf("%d.0.0", config.ConfigFile.Version.Major())

	g := &Gossip{
		ctx:              ctx,
		pubsub:           nil,
		serverNode:       nil,
		clientNode:       nil,
		discoveryHandler: discoveryHandler,
		version:          version,
		mx:               &sync.RWMutex{},
		subs:             []*pubsub.Subscription{},
		topics:           map[string]*pubsub.Topic{},

		isRunning: new(atomic.Bool),
	}

	return g
}

func (g *Gossip) Run(
	serverNode PubsubServerNodeConnector, clientNode PubsubClientNodeStreamer,
	authRepo PubsubAuthStorer, followRepo PubsubFollowingStorer,
) {
	if g.isRunning.Load() {
		return
	}
	g.clientNode = clientNode
	g.serverNode = serverNode

	if err := g.run(serverNode); err != nil {
		log.Fatalf("failed to create Gossip sub: %s", err)
	}

	if err := g.preSubscribe(authRepo, followRepo); err != nil {
		log.Fatalf("failed to presubscribe: %s", err)
	}
	for {
		if !g.isRunning.Load() {
			return
		}

		g.mx.RLock()
		subscriptions := g.subs
		g.mx.RUnlock()

		for _, sub := range subscriptions {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			msg, err := sub.Next(ctx)
			if errors.Is(err, context.DeadlineExceeded) {
				cancel()
				continue
			}
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Infof("pubsub discovery: subscription: %v", err)
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
						log.Infof("pubsub discovery: user update error: %v", err)
					}
				}
			}
		}
	}
}

func (g *Gossip) run(n PubsubServerNodeConnector) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("pubsub discovery: recovered from panic: %v", r)
			err = errors.New("recovered from panic")
		}
	}()
	if g == nil {
		panic("discovery service not initialized properly")
	}

	g.pubsub, err = pubsub.NewGossipSub(g.ctx, n.Node())
	if err != nil {
		return err
	}
	g.isRunning.Store(true)
	log.Infoln("started pubsub service")

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

func (g *Gossip) preSubscribe(authRepo PubsubAuthStorer, followRepo PubsubFollowingStorer) error {
	if authRepo == nil {
		return nil
	}
	var (
		nextCursor string
		limit      = uint64(20)
	)
	g.owner = authRepo.GetOwner()

	for {
		followees, cur, _ := followRepo.GetFollowees(g.owner.UserId, &limit, &nextCursor)
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
	log.Infoln("followees presubscribed")
	return nil
}

func (g *Gossip) Close() (err error) {
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

	var errs []error
	for _, topic := range g.topics {
		if err = topic.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	g.isRunning.Store(false)
	g.pubsub = nil
	return
}

// PublishOwnerUpdate - publish for followers
func (g *Gossip) PublishOwnerUpdate(ownerId string, msg event.Message) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub service not initialized")
	}
	topicName := fmt.Sprintf(userUpdateTopicPrefix, g.version, ownerId)
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

	var buf = bytes.NewBuffer(nil)
	encoder := msgpack.NewEncoder(buf)
	encoder.SetCustomStructTag("json")
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("client stream encoding: %w", err)
	}

	err = topic.Publish(g.ctx, buf.Bytes())
	if err != nil {
		log.Errorf("pubsub discovery: failed to publish message: %v", err)
		return err
	}
	return
}

func (g *Gossip) GenericSubscribe(topicName string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub service not initialized")
	}
	if topicName == "" {
		return errors.New("can't subscribe to own user")
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
	log.Infof("pubsub discovery: subscribed to topic: %s", topicName)
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()
	return nil
}

// SubscribeUserUpdate - follow someone
func (g *Gossip) SubscribeUserUpdate(userId string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub service not initialized")
	}
	if g.owner.UserId == userId {
		return errors.New("can't subscribe to own user")
	}

	topicName := fmt.Sprintf(userUpdateTopicPrefix, g.version, userId)
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
	log.Infof("pubsub discovery: subscribed to topic: %s", topicName)
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()
	return nil
}

// UnsubscribeUserUpdate - unfollow someone
func (g *Gossip) UnsubscribeUserUpdate(userId string) (err error) {
	if g == nil || !g.isRunning.Load() {
		return errors.New("pubsub service not initialized")
	}
	topicName := fmt.Sprintf(userUpdateTopicPrefix, g.version, userId)
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

func (g *Gossip) handleUserUpdate(msg *pubsub.Message) error {
	var simulatedMessage event.Message
	if err := msgpack.Unmarshal(msg.Data, &simulatedMessage); err != nil {
		log.Errorf("pubsub discovery: failed to decode discovery message: %v %s", err, msg.Data)
		return err
	}
	if simulatedMessage.NodeId == g.serverNode.ID().String() {
		return nil
	}

	if simulatedMessage.Path == "" {
		log.Errorln("pubsub user update: message has no path", simulatedMessage.Path)
		return fmt.Errorf("pubsub user update: message has no path: %s", string(msg.Data))
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
		g.serverNode.ID().String(),
		simulatedMessage.Path,
		*simulatedMessage.Body,
	)
	return err
}

func (g *Gossip) handlePubSubDiscovery(msg *pubsub.Message) {
	var discoveryMsg warpnet.WarpAddrInfo
	if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
		log.Errorf("pubsub discovery: failed to decode discovery message: %v %s", err, msg.Data)
		return
	}
	if discoveryMsg.ID == "" {
		log.Errorf("pubsub discovery: message has no ID: %s", string(msg.Data))
		return
	}
	if discoveryMsg.ID == g.serverNode.ID() {
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

	if g.discoveryHandler == nil { // just bootstrap
		if err := g.serverNode.Connect(peerInfo); err != nil {
			log.Errorf(
				"pubsub discovery: failed to connect to peer %s: %v",
				peerInfo.String(),
				err,
			)
			return
		}
		log.Debugf("pubsub: connected to peer: %s %s", discoveryMsg.Addrs, discoveryMsg.ID)
		return
	}
	g.discoveryHandler(peerInfo) // add new user
}

func (g *Gossip) publishPeerInfo(discTopic *pubsub.Topic) {
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
			addrs := make([]string, 0, len(g.serverNode.Addrs()))
			for _, addr := range g.serverNode.Addrs() {
				if addr == "" {
					continue
				}
				addrs = append(addrs, addr)
			}

			msg := warpnet.WarpAddrInfo{
				ID:    g.serverNode.ID(),
				Addrs: addrs,
			}
			var buf = bytes.NewBuffer(nil)
			encoder := msgpack.NewEncoder(buf)
			encoder.SetCustomStructTag("json")
			if err := encoder.Encode(msg); err != nil {
				log.Errorf("client stream encoding: %v", err)
				return
			}

			err := discTopic.Publish(g.ctx, buf.Bytes())
			if err != nil {
				log.Errorf("pubsub discovery: failed to publish message: %v", err)
			}
		}
	}
}
