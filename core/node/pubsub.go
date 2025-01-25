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
	// Настраиваем PubSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub: %s", err)
	}

	// defaultly enabled topic
	topic, err := ps.Join(discoveryTopic)
	if err != nil {
		return nil, fmt.Errorf("failed to join discovery topic: %s", err)
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
	g.topics[topic.String()] = topic

	go func() {
		if err := g.subscribeToDiscovery(topic); err != nil {
			log.Printf("failed to subscribe to discovery topic: %v", err)
		}
	}()

	return g, g.publishPeerInfo(topic)
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

func (g *Gossip) subscribeToDiscovery(topic *pubsub.Topic) error {
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %s", err)
	}
	g.mx.Lock()
	g.subs = append(g.subs, sub)
	g.mx.Unlock()

	for {
		msg, err := sub.Next(g.ctx)
		if err != nil {
			return fmt.Errorf("subscription error: %v", err)
		}

		var discoveryMsg types.WarpAddrInfo
		if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
			return fmt.Errorf("failed to decode discovery message: %w %s", err, msg.Data)
		}
		if discoveryMsg.ID == "" {
			log.Println("discovery message has no ID", string(msg.Data))
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

		if g.discovery == nil { // just bootstrap
			if err := g.node.Connect(g.ctx, peerInfo); err != nil {
				return fmt.Errorf("failed to connect to peer: %w", err)
			}
			log.Printf("connected to peer: %s", discoveryMsg.ID)
			return nil
		}
		g.discovery.HandlePeerFound(peerInfo) // add new user

		// publish again
		for _, t := range g.topics {
			if err := g.publishPeerInfo(t); err != nil {
				log.Println(err)
			}
		}
	}
}

func (g *Gossip) publishPeerInfo(topic *pubsub.Topic) error {
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
		return err
	}
	err = topic.Publish(g.ctx, data)
	if err != nil {
		return fmt.Errorf("failed to publish peer info: %w", err)
	}
	log.Printf("published peer info: %s, to topic: %s", data, topic.String())
	return nil
}
