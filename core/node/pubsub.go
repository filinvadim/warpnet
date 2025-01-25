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
	"time"
)

type PeerInfoStorer interface {
	types.P2PNode
}

const discoveryTopic = "peer-discovery"

type Gossip struct {
	ctx       context.Context
	pubsub    *pubsub.PubSub
	node      PeerInfoStorer
	discovery types.WarpDiscoveryHandler

	mx     *sync.RWMutex
	subs   []*pubsub.Subscription
	topics map[string]*pubsub.Topic
}

func NewPubSub(ctx context.Context, h PeerInfoStorer, discovery types.WarpDiscoveryHandler) (*Gossip, error) {
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub: %s", err)
	}

	g := &Gossip{
		ctx:       ctx,
		pubsub:    ps,
		node:      h,
		discovery: discovery,
		subs:      []*pubsub.Subscription{},
		topics:    map[string]*pubsub.Topic{},
		mx:        &sync.RWMutex{},
	}

	go func() {
		if err := g.runDiscovery(); err != nil {
			log.Printf("failed to subscribe to discovery topic: %v", err)
		}
	}()

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

	for _, sub := range g.subs {
		sub.Cancel()
	}
	for _, topic := range g.topics {
		if err = topic.Close(); err != nil {
			return err
		}
	}

	g.pubsub = nil
	return
}

func (g *Gossip) runDiscovery() error {
	topic, err := g.pubsub.Join(discoveryTopic)
	if err != nil {
		return err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("pubsub discovery: failed to subscribe to topic: %s", err)
	}
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()

	go func() {
		if err := g.publishPeerInfo(topic); err != nil {
			log.Println(err)
		}
	}()

	for {
		msg, err := sub.Next(g.ctx)
		if err != nil {
			return fmt.Errorf("pubsub discovery: subscription error: %v", err)
		}

		var discoveryMsg types.WarpAddrInfo
		if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
			return fmt.Errorf("pubsub discovery: failed to decode discovery message: %w %s", err, msg.Data)
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
			log.Println("pubsub discovery: peer has already been registered", discoveryMsg.ID)
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

		if g.discovery == nil { // just bootstrap
			log.Println("pubsub discovery: no discovery handler")
			if err := g.node.Connect(g.ctx, peerInfo); err != nil {
				return fmt.Errorf("pubsub discovery: failed to connect to peer: %w", err)
			}
			log.Printf("pubsub discovery: connected to peer: %s", discoveryMsg.ID)
			return nil
		}
		g.discovery.HandlePeerFound(peerInfo) // add new user
	}

}

func (g *Gossip) publishPeerInfo(topic *pubsub.Topic) error {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
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
				return err
			}
			err = topic.Publish(g.ctx, data)
			if err != nil {
				log.Printf("pubsub discovery: failed to publish message: %v", err)
				return fmt.Errorf("pubsub discovery: failed to publish peer info: %w", err)
			}
			log.Printf("pubsub discovery: published peer info to topic: %s", topic.String())
		}
	}
}
