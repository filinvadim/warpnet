package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	node2 "github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type PeerInfoStorer interface {
	Node() warpnet.P2PNode
	Connect(warpnet.PeerAddrInfo) error
	ID() warpnet.WarpPeerID
	Addrs() []string
}

type Gossip struct {
	ctx              context.Context
	pubsub           *pubsub.PubSub
	node             PeerInfoStorer
	discoveryHandler node2.DiscoveryHandler

	mx     *sync.RWMutex
	subs   []*pubsub.Subscription
	topics map[string]*pubsub.Topic

	isRunning *atomic.Bool
}

func NewPubSub(ctx context.Context, discoveryHandler node2.DiscoveryHandler) *Gossip {
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

	g.pubsub = nil
	g.isRunning.Store(false)
	return
}

const discoveryTopic = "peer-discovery"

func (g *Gossip) RunDiscovery(n PeerInfoStorer) {
	if g == nil {
		panic("discovery service not initialized properly")
	}

	var err error
	g.pubsub, err = pubsub.NewGossipSub(g.ctx, n.Node())
	if err != nil {
		log.Fatalf("failed to create Gossip sub: %s", err)
	}
	g.node = n

	if g.isRunning.Load() {
		return
	}
	g.isRunning.Store(true)

	discTopic, err := g.pubsub.Join(discoveryTopic)
	if err != nil {
		log.Printf("failed to join discovery topic: %s", err)
		return
	}

	discoverySub, err := discTopic.Subscribe()
	if err != nil {
		log.Printf("pubsub discovery: failed to subscribe to topic: %s", err)
		return
	}
	g.mx.Lock()
	g.subs = append(g.subs, discoverySub)
	g.mx.Unlock()

	go g.publishPeerInfo(discTopic)

	for {
		g.mx.RLock()
		subscriptions := g.subs
		g.mx.RUnlock()

		for _, sub := range subscriptions {
			ctx, cancelF := context.WithTimeout(g.ctx, time.Second*5)
			msg, err := sub.Next(ctx)
			if errors.Is(err, context.DeadlineExceeded) {
				cancelF()
				continue
			}
			cancelF()

			if isContextCancelledError(err) {
				log.Printf("pubsub discovery stopped by context")
				return
			}
			if err != nil {
				log.Printf("pubsub discovery: subscription error: %v", err)
				return
			}

			var discoveryMsg warpnet.WarpAddrInfo
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

			peerInfo := warpnet.PeerAddrInfo{
				ID:    discoveryMsg.ID,
				Addrs: make([]multiaddr.Multiaddr, 0, len(discoveryMsg.Addrs)),
			}

			for _, addr := range discoveryMsg.Addrs {
				ma, _ := multiaddr.NewMultiaddr(addr)
				peerInfo.Addrs = append(peerInfo.Addrs, ma)
			}

			if g.discoveryHandler == nil { // just bootstrap
				log.Println("pubsub: no discovery handler")
				if err := g.node.Connect(peerInfo); err != nil {
					log.Printf("pubsub discovery: failed to connect to peer: %v", err)
					continue
				}
				log.Printf("pubsub: connected to peer: %s", discoveryMsg.ID)
				continue
			}
			g.discoveryHandler(peerInfo) // add new user
		}
	}
}

func (g *Gossip) publishPeerInfo(discTopic *pubsub.Topic) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	defer log.Println("pubsub: publisher stopped")

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
				addrs = append(addrs, addr)
			}

			msg := warpnet.WarpAddrInfo{
				ID:    g.node.ID(),
				Addrs: addrs,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("pubsub discovery: failed to marchal message: %v", err)
				return
			}
			err = discTopic.Publish(g.ctx, data)
			if err != nil {
				log.Printf("pubsub discovery: failed to publish message: %v", err)
			}
		}
	}
}

func isContextCancelledError(err error) bool {
	switch {
	case err == nil:
		return false
	//goland:noinspection ALL
	case err == context.Canceled:
		return true
	case errors.Is(err, context.Canceled):
		return true
	case strings.Contains(err.Error(), "context canceled"):
		return true
	}
	return false
}
