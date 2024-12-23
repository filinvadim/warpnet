package node

import (
	"context"
	"encoding/json"
	"github.com/filinvadim/warpnet/database"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"log"
	"time"
)

const NetworkName = "warpnet"

var publicRelays = []string{
	"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWJ8S8B118DVjMrAzihsXFvGCGZeYThHH9y9MWanoPuNi1",
}

type Node struct {
	ID           string
	nodeRepo     *database.NodeRepo
	authRepo     *database.AuthRepo
	userRepo     *database.UserRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
	followRepo   *database.FollowRepo
	replyRepo    *database.RepliesRepo

	node  host.Host
	mdns  mdns.Service
	relay *relayv2.Relay
}

//for _, maddr := range n.bootstrapPeers {
//peerInfo, _ := peer.AddrInfoFromP2pAddr(maddr)
//if err := n.node.Connect(ctx, *peerInfo); err != nil {
//log.Printf("Failed to connect to bootstrap node: %s", err)
//} else {
//log.Printf("Connected to bootstrap node: %s", peerInfo.ID)
//}
//}

func NewNode(
	ctx context.Context,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	timelineRepo *database.TimelineRepo,
	followRepo *database.FollowRepo,
	replyRepo *database.RepliesRepo,
) (*Node, error) {
	privKey := authRepo.PrivateKey()

	store, err := pstoreds.NewPeerstore(ctx, nodeRepo, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}
	manager, err := connmgr.NewConnManager(100, 400, connmgr.WithGracePeriod(time.Minute*2))
	if err != nil {
		return nil, err
	}
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()))
	if err != nil {
		return nil, err
	}

	var relays []peer.AddrInfo
	for _, addr := range publicRelays {
		ai, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		relays = append(relays, *ai)
	}

	providersCache := NewProviderCache(ctx, nodeRepo)
	node, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/4001",
			"/ip4/0.0.0.0/tcp/8081/ws",
			"/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
			"/ip6/::/tcp/4001",
			"/ip6/::/udp/4001/quic-v1/webtransport",
		),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(ws.New),
		libp2p.Identity(privKey.(crypto.PrivKey)),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork([]byte(NetworkName)),
		libp2p.UserAgent(NetworkName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(relays),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			return setupPrivateDHT(ctx, h, nodeRepo, providersCache)
		}),
	)
	if err != nil {
		return nil, err
	}
	mdnsService := mdns.NewMdnsService(node, NetworkName, &discoveryNotifee{node})
	relay, err := relayv2.New(node)
	if err != nil {
		return nil, err
	}

	n := &Node{
		node.ID().String(),
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		timelineRepo,
		followRepo,
		replyRepo,
		node,
		mdnsService,
		relay,
	}
	return n, nil
}

func (n *Node) GetID() string {
	return n.ID
}

func (n *Node) Stop() error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered:", r)
		}
	}()
	if n.mdns != nil {
		_ = n.mdns.Close()
	}
	if n.relay != nil {
		_ = n.relay.Close()
	}
	_ = n.nodeRepo.Close()

	return n.node.Close()
}

func setupPrivateDHT(
	ctx context.Context, h host.Host, repo *database.NodeRepo, providerStore providers.ProviderStore,
) (*dht.IpfsDHT, error) {
	kdht, err := dht.New(
		ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/"+NetworkName),
		dht.Datastore(repo),
		dht.BootstrapPeers(),
		dht.ProviderStore(providerStore),
	)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := kdht.Bootstrap(ctx); err != nil {
			log.Printf("failed to bootstrap kdht: %v", err)
		}
	}()
	go monitorDHT(kdht)
	return kdht, nil
}

func monitorDHT(idht *dht.IpfsDHT) {
	for {
		time.Sleep(50 * time.Second)
		peers := idht.RoutingTable().ListPeers()
		log.Printf("DHT routing table contains %d peers", len(peers))
	}
}

type discoveryNotifee struct {
	host host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("Found new peer: %s", pi.ID.String())
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

func registerHandler(h host.Host, protocolID protocol.ID) {
	h.SetStreamHandler(protocolID, func(s network.Stream) {
		defer s.Close()
		log.Println("New stream opened")

		buf := make([]byte, 1024)
		n, err := s.Read(buf)
		if err != nil {
			log.Printf("Error reading from stream: %s", err)
			return
		}
		log.Printf("Received message: %s", string(buf[:n]))

		// Отправляем ответ
		_, err = s.Write([]byte("Hello, client!"))
		if err != nil {
			log.Printf("Error writing to stream: %s", err)
			return
		}
	})
}

func sendMessage(h host.Host, peerID peer.ID, protocol protocol.ID, message string) {
	// Устанавливаем соединение
	s, err := h.NewStream(context.Background(), peerID, protocol)
	if err != nil {
		log.Printf("Failed to create stream: %s", err)
		return
	}
	defer s.Close()

	// Отправляем сообщение
	_, err = s.Write([]byte(message))
	if err != nil {
		log.Printf("Failed to write to stream: %s", err)
		return
	}

	// Читаем ответ
	buf := make([]byte, 1024)
	n, err := s.Read(buf)
	if err != nil {
		log.Printf("Error reading from stream: %s", err)
		return
	}
	log.Printf("Response: %s", string(buf[:n]))
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
