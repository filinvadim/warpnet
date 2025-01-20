package node

import (
	"context"
	"encoding/json"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"
	"strings"
	"sync"
	"time"
)

type discoveryNotifee struct {
	host host.Host
}

func NewDiscoveryNotifee(h host.Host) *discoveryNotifee {
	return &discoveryNotifee{h}
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("Found new peer: %s %v", pi.ID.String(), pi.Addrs)
	if err := n.host.Connect(context.Background(), pi); err != nil {
		log.Printf("Failed to connect to new peer: %s", err)
	} else {
		log.Printf("Connected to new peer: %s", pi.ID)
	}
}

const discoveryTopic = "peer-discovery"

type Gossip struct {
	ctx      context.Context
	pubsub   *pubsub.PubSub
	node     host.Host
	stopChan chan struct{}
	tick     *time.Ticker
	subs     []*pubsub.Subscription
	topics   map[string]*pubsub.Topic

	mx         *sync.RWMutex
	knownPeers map[peer.ID]struct{}
}

func NewPubSub(ctx context.Context, h host.Host) (*Gossip, error) {
	// Настраиваем PubSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub: %s", err)
	}

	// Подключаемся к топику
	topic, err := ps.Join(discoveryTopic)
	if err != nil {
		return nil, fmt.Errorf("failed to join discovery topic: %s", err)
	}

	g := &Gossip{
		ctx:        ctx,
		pubsub:     ps,
		node:       h,
		stopChan:   make(chan struct{}),
		tick:       time.NewTicker(time.Minute),
		subs:       []*pubsub.Subscription{},
		topics:     map[string]*pubsub.Topic{},
		mx:         &sync.RWMutex{},
		knownPeers: map[peer.ID]struct{}{},
	}
	g.topics[topic.String()] = topic

	go func() {
		if err := g.subscribeToDiscovery(topic); err != nil {
			log.Printf("failed to subscribe to discovery topic: %v", err)
		}
	}()
	go func() {
		if err := g.publishPeerInfo(topic); err != nil {
			log.Printf("failed to publish peer info to discovery topic: %v", err)
		}
	}()
	return g, nil
}

func (g *Gossip) Close() (err error) {
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
	g.tick.Stop()
	close(g.stopChan)
	g.pubsub = nil
	return
}

type DiscoveryMessage struct {
	PeerID string   `json:"peer_id"`
	Addrs  []string `json:"addrs"`
}

func (g *Gossip) subscribeToDiscovery(topic *pubsub.Topic) error {
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %s", err)
	}
	g.subs = append(g.subs, sub)

	for {
		msg, err := sub.Next(g.ctx)
		if err != nil {
			return fmt.Errorf("subscription error: %v", err)
		}

		var discoveryMsg DiscoveryMessage
		if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
			return fmt.Errorf("failed to decode discovery message: %w", err)
		}

		for _, addr := range discoveryMsg.Addrs {
			if addr == "" {
				log.Println("discovery message address is empty", addr)
				continue
			}
			if strings.Contains(addr, "/p2p-circuit") {
				continue
			}
			if discoveryMsg.PeerID == g.node.ID().String() {
				continue
			}

			addr = fmt.Sprintf("%s/p2p/%s", addr, discoveryMsg.PeerID)
			peerInfo, err := peer.AddrInfoFromString(addr)
			if err != nil || peerInfo == nil {
				return fmt.Errorf("failed to parse peer info: %w", err)
			}
			if _, ok := g.knownPeers[peerInfo.ID]; ok {
				continue
			}
			if err := g.node.Connect(g.ctx, *peerInfo); err != nil {
				return fmt.Errorf("failed to connect to peer: %w", err)
			}
			log.Printf("connected to peer: %s", discoveryMsg.PeerID)
			g.knownPeers[peerInfo.ID] = struct{}{}
		}
	}
}

func (g *Gossip) publishPeerInfo(topic *pubsub.Topic) error {
	for {
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
		case <-g.stopChan:
			return nil
		case <-g.tick.C:
			msg := DiscoveryMessage{
				PeerID: g.node.ID().String(),
				Addrs:  convertAddrs(g.node.Addrs()),
			}
			data, err := json.Marshal(msg)
			if err != nil {
				return err
			}
			err = topic.Publish(g.ctx, data)
			if err != nil {
				return fmt.Errorf("failed to publish peer info: %w", err)
			}
		}
	}
}

func convertAddrs(maddrs []multiaddr.Multiaddr) []string {
	var addrs []string
	for _, maddr := range maddrs {
		addrs = append(addrs, maddr.String())
	}
	return addrs
}
