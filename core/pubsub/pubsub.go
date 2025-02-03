package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/event-gen"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pubSubDiscoveryTopic = "peer-discovery"
)

const (
	userUpdateTopicPrefix = "user-update"
)

type PeerInfoStorer interface {
	Node() warpnet.P2PNode
	Connect(warpnet.PeerAddrInfo) error
	ID() warpnet.WarpPeerID
	Addrs() []string
	GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error)
}

type Gossip struct {
	ctx              context.Context
	pubsub           *pubsub.PubSub
	node             PeerInfoStorer
	discoveryHandler discovery.DiscoveryHandler

	mx     *sync.RWMutex
	subs   []*pubsub.Subscription
	topics map[string]*pubsub.Topic

	isRunning *atomic.Bool
}

func NewPubSub(ctx context.Context, discoveryHandler discovery.DiscoveryHandler) *Gossip {
	g := &Gossip{
		ctx:              ctx,
		pubsub:           nil,
		node:             nil,
		discoveryHandler: discoveryHandler,

		mx:     &sync.RWMutex{},
		subs:   []*pubsub.Subscription{},
		topics: map[string]*pubsub.Topic{},

		isRunning: new(atomic.Bool),
	}

	return g
}

func (g *Gossip) Run(n PeerInfoStorer) {
	if g.isRunning.Load() {
		return
	}
	if err := g.run(n); err != nil {
		log.Fatalf("failed to create Gossip sub: %s", err)
	}
	for {
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
				log.Infof("pubsub discovery: subscription error: %v", err)
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

func (g *Gossip) run(n PeerInfoStorer) error {
	if g == nil {
		panic("discovery service not initialized properly")
	}

	var err error
	g.pubsub, err = pubsub.NewGossipSub(g.ctx, n.Node())
	if err != nil {

	}
	g.node = n
	g.isRunning.Store(true)
	log.Infoln("started pubsub service")

	discTopic, err := g.pubsub.Join(pubSubDiscoveryTopic)
	if err != nil {
		log.Errorf("failed to join discovery topic: %s", err)
		return err
	}

	if _, err = discTopic.Relay(); err != nil {
		log.Errorf("failed to relay discovery topic: %s", err)
	}

	discoverySub, err := discTopic.Subscribe()
	if err != nil {
		log.Errorf("pubsub discovery: failed to subscribe to topic: %s", err)
		return err
	}
	g.mx.Lock()
	g.subs = append(g.subs, discoverySub)
	g.mx.Unlock()

	go g.publishPeerInfo(discTopic)
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
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, ownerId)
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
		log.Errorf("pubsub discovery: failed to marchal message: %v", err)
		return err
	}
	err = topic.Publish(g.ctx, data)
	if err != nil {
		log.Errorf("pubsub discovery: failed to publish message: %v", err)
		return err
	}
	return
}

// SubscribeUserUpdate - follow someone
func (g *Gossip) SubscribeUserUpdate(userId string) (err error) {
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, userId)
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

	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()
	return nil
}

// UnsubscribeUserUpdate - unfollow someone
func (g *Gossip) UnsubscribeUserUpdate(userId string) (err error) {
	topicName := fmt.Sprintf("%s-%s", userUpdateTopicPrefix, userId)
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
	if err := json.Unmarshal(msg.Data, &simulatedMessage); err != nil {
		log.Errorf("pubsub discovery: failed to decode discovery message: %v %s", err, msg.Data)
		return err
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
	_, err := g.node.GenericStream( // send to self
		g.node.ID().String(),
		stream.WarpRoute(simulatedMessage.Path),
		msg.Data,
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
		log.Errorf("pubsub discovery: message has no ID", string(msg.Data))
		return
	}
	if discoveryMsg.ID == g.node.ID() {
		return
	}

	peerInfo := warpnet.PeerAddrInfo{
		ID:    discoveryMsg.ID,
		Addrs: make([]multiaddr.Multiaddr, 0, len(discoveryMsg.Addrs)),
	}

	for _, addr := range discoveryMsg.Addrs {
		ma, _ := multiaddr.NewMultiaddr(addr)
		peerInfo.Addrs = append(peerInfo.Addrs, ma)
	}

	if g.discoveryHandler == nil { // just bootstrap
		if err := g.node.Connect(peerInfo); err != nil {
			log.Errorf("pubsub discovery: failed to connect to peer: %v", err)
			return
		}
		log.Infof("pubsub: connected to peer: %s", discoveryMsg.ID)
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
			log.Infoln(g.ctx.Err())
			return
		case <-ticker.C:
			addrs := make([]string, 0, len(g.node.Addrs()))
			for _, addr := range g.node.Addrs() {
				if addr == "" {
					continue
				}
				addrs = append(addrs, addr)
			}

			msg := warpnet.WarpAddrInfo{
				ID:    g.node.ID(),
				Addrs: addrs,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Errorf("pubsub discovery: failed to marchal message: %v", err)
				return
			}
			err = discTopic.Publish(g.ctx, data)
			if err != nil {
				log.Errorf("pubsub discovery: failed to publish message: %v", err)
			}
		}
	}
}
