package node

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filinvadim/warpnet/core/types"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type NodeWrapper interface {
	Node() types.P2PNode
}

type PeerInfoStorer interface {
	ID() types.WarpPeerID
	Peerstore() types.WarpPeerstore
	Connect(context.Context, types.PeerAddrInfo) error
	Addrs() []multiaddr.Multiaddr
}

type Gossip struct {
	ctx              context.Context
	pubsub           *pubsub.PubSub
	node             PeerInfoStorer
	discoveryHandler DiscoveryHandler

	mx     *sync.RWMutex
	subs   []*pubsub.Subscription
	topics map[string]*pubsub.Topic

	isRunning *atomic.Bool
}

func NewPubSub(ctx context.Context, h NodeWrapper, discoveryHandler DiscoveryHandler) (*Gossip, error) {
	ps, err := pubsub.NewGossipSub(ctx, h.Node())
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub: %s", err)
	}

	g := &Gossip{
		ctx:              ctx,
		pubsub:           ps,
		node:             h.Node(),
		discoveryHandler: discoveryHandler,

		mx:     &sync.RWMutex{},
		subs:   []*pubsub.Subscription{},
		topics: map[string]*pubsub.Topic{},

		isRunning: new(atomic.Bool),
	}

	return g, nil
}

func (g *Gossip) Close() (err error) {
	g.mx.Lock()
	defer g.mx.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	if !g.isRunning.Load() {
		return
	}
	for _, sub := range g.subs {
		sub.Cancel()
	}
	for _, topic := range g.topics {
		if err = topic.Close(); err != nil {
			return err
		}
	}

	g.pubsub = nil
	g.isRunning.Store(false)
	return
}

const discoveryTopic = "peer-discovery"

func (g *Gossip) RunDiscovery() {
	if g == nil || g.pubsub == nil || g.node == nil {
		panic("discovery service not initialized properly")
	}
	if g.isRunning.Load() {
		return
	}
	topic, err := g.pubsub.Join(discoveryTopic)
	if err != nil {
		log.Printf("failed to join discovery topic: %s", err)
		return
	}

	sub, err := topic.Subscribe()
	if err != nil {
		log.Printf("pubsub discovery: failed to subscribe to topic: %s", err)
		return
	}
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()

	go g.publishPeerInfo(topic)

	for {
		msg, err := sub.Next(g.ctx)
		if err != nil {
			log.Printf("pubsub discovery: subscription error: %v", err)
			return
		}

		var discoveryMsg types.WarpAddrInfo
		if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
			log.Printf("pubsub discovery: failed to decode discovery message: %v %s", err, msg.Data)
			continue
		}
		if discoveryMsg.ID == "" {
			log.Println("pubsub discovery: message has no ID", string(msg.Data))
			continue
		}
		if discoveryMsg.ID == g.node.ID() {
			continue
		}
		existedPeer := g.node.Peerstore().PeerInfo(discoveryMsg.ID)
		if existedPeer.ID != "" {
			continue
		}
		peerInfo := types.PeerAddrInfo{
			ID:    discoveryMsg.ID,
			Addrs: make([]multiaddr.Multiaddr, 0, len(discoveryMsg.Addrs)),
		}

		for _, addr := range discoveryMsg.Addrs {
			ma, _ := multiaddr.NewMultiaddr(addr)
			peerInfo.Addrs = append(peerInfo.Addrs, ma)
		}

		if g.discoveryHandler == nil { // just bootstrap
			log.Println("pubsub: no discovery handler")
			if err := g.node.Connect(g.ctx, peerInfo); err != nil {
				log.Printf("pubsub discovery: failed to connect to peer: %v", err)
				continue
			}
			log.Printf("pubsub: connected to peer: %s", discoveryMsg.ID)
			continue
		}
		g.discoveryHandler(peerInfo) // add new user
	}

}

func (g *Gossip) publishPeerInfo(topic *pubsub.Topic) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		if !g.isRunning.Load() {
			return
		}
		select {
		case <-g.ctx.Done():
			log.Println(g.ctx.Err())
			return
		case <-ticker.C:
			addrs := make([]string, 0, len(g.node.Addrs()))
			for _, addr := range g.node.Addrs() {
				addrs = append(addrs, addr.String())
			}
			msg := types.WarpAddrInfo{
				ID:    g.node.ID(),
				Addrs: addrs,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("pubsub discovery: failed to marchal message: %v", err)
				return
			}
			err = topic.Publish(g.ctx, data)
			if err != nil {
				log.Printf("pubsub discovery: failed to publish message: %v", err)
			}
		}
	}
}
