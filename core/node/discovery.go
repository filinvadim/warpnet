package node

import (
	"context"
	"encoding/json"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"time"
)

type discoveryNotifee struct {
	host host.Host
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

type DiscoveryMessage struct {
	PeerID string   `json:"peer_id"`
	Addrs  []string `json:"addrs"`
}

func subscribeToDiscovery(ctx context.Context, topic *pubsub.Topic, h host.Host) {
	sub, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %s", err)
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			log.Printf("Subscription error: %s", err)
			continue
		}

		// Обработка сообщения
		var discoveryMsg DiscoveryMessage
		if err := json.Unmarshal(msg.Data, &discoveryMsg); err != nil {
			log.Printf("Failed to decode discovery message: %s", err)
			continue
		}

		// Подключение к узлу
		peerInfo, err := peer.AddrInfoFromString(discoveryMsg.Addrs[0])
		if err != nil {
			log.Printf("Failed to parse address: %s", err)
			continue
		}
		if err := h.Connect(ctx, *peerInfo); err != nil {
			log.Printf("Failed to connect to peer: %s", err)
		} else {
			log.Printf("Connected to peer: %s", discoveryMsg.PeerID)
		}
	}
}

func publishPeerInfo(ctx context.Context, topic *pubsub.Topic, peerID string, addrs []string) error {
	msg := DiscoveryMessage{
		PeerID: peerID,
		Addrs:  addrs,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return topic.Publish(ctx, data)
}

func newPubSub(ctx context.Context, h host.Host) {
	// Настраиваем PubSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Fatalf("Failed to create PubSub: %s", err)
	}

	// Подключаемся к топику
	topic, err := ps.Join(discoveryTopic)
	if err != nil {
		log.Fatalf("Failed to join discovery topic: %s", err)
	}

	// Подписываемся на сообщения
	go subscribeToDiscovery(ctx, topic, h)

	// Публикуем информацию об узле каждые 10 секунд
	for {
		addrs := make([]string, 0)
		for _, addr := range h.Addrs() {
			addrs = append(addrs, addr.String())
		}
		err := publishPeerInfo(ctx, topic, h.ID().String(), addrs)
		if err != nil {
			log.Printf("Failed to publish peer info: %s", err)
		}
		time.Sleep(10 * time.Second)
	}
}
