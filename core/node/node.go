package node

import (
	"context"
	"crypto/rand"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/database"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
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
	"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWJ8S8B118DVjMrAzihsXFvGCGZeYThHH9y9MWanoPuNi1", // just a stub
}

type Node struct {
	id        string
	addresses []string
	nodeRepo  *database.NodeRepo
	authRepo  *database.AuthRepo

	node  host.Host
	mdns  mdns.Service
	relay *relayv2.Relay
}

//for _, maddr := range n.bootstrapPeers {
//peerInfo, _ := peer.AddrInfoFromP2pAddr(maddr)
//if err := n.core.Connect(ctx, *peerInfo); err != nil {
//log.Printf("Failed to connect to bootstrap core: %s", err)
//} else {
//log.Printf("Connected to bootstrap core: %s", peerInfo.ID)
//}
//}

func NewNode(
	ctx context.Context,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	isBootstrap bool,
) (_ *Node, err error) {
	_ = logging.SetLogLevel("*", "warn")
	privKey := authRepo.PrivateKey()

	var (
		store          peerstore.Peerstore
		providersCache providers.ProviderStore
	)
	if isBootstrap {
		seed := make([]byte, 8)
		_, _ = rand.Read(seed)
		privKey, err = encrypting.GenerateKeyFromSeed(seed)
		if err != nil {
			return nil, err
		}
		store, err = pstoremem.NewPeerstore()
		if err != nil {
			return nil, err
		}
		mapStore := datastore.NewMapDatastore()
		providersCache, err = providers.NewProviderManager("bootstrap", store, mapStore)
		if err != nil {
			return nil, err
		}
	} else {
		store, err = pstoreds.NewPeerstore(ctx, nodeRepo, pstoreds.DefaultOpts())
		if err != nil {
			return nil, err
		}
		providersCache, err = NewProviderCache(ctx, nodeRepo)
		if err != nil {
			return nil, err
		}
	}

	limiter := rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale())

	manager, err := connmgr.NewConnManager(
		100,
		limiter.GetConnLimits().GetConnTotalLimit(),
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, err
	}

	rm, err := rcmgr.NewResourceManager(limiter)
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

	listenAddrs := []string{
		"/ip4/0.0.0.0/tcp/4001",
		"/ip4/0.0.0.0/tcp/443/ws",
		"/ip6/::/tcp/4001",
	}

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddrs...),
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

	var addresses = make([]string, 0, len(listenAddrs))
	for _, addr := range node.Addrs() {
		addresses = append(addresses, addr.String())
	}

	n := &Node{
		node.ID().String(),
		addresses,
		nodeRepo,
		authRepo,
		node,
		mdnsService,
		relay,
	}
	log.Printf("NODE STARTED WITH ID %s AND ADDRESSES %v\n", n.ID(), n.Addresses())
	return n, nil
}

func (n *Node) ID() string {
	return n.id
}

func (n *Node) Addresses() []string {
	return n.addresses
}

func (n *Node) Stop() error {
	log.Println("shutting down node...")
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered: %v\n", r)
		}
	}()
	if n.mdns != nil {
		_ = n.mdns.Close()
	}
	if n.relay != nil {
		_ = n.relay.Close()
	}
	if n.nodeRepo != nil {
		_ = n.nodeRepo.Close()
	}

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
			log.Printf("failed to bootstrap kdht: %v\n", err)
		}
	}()
	go monitorDHT(kdht)
	if err = <-kdht.RefreshRoutingTable(); err != nil {
		log.Printf("failed to refresh kdht routing table: %v\n", err)
	}
	return kdht, nil
}

func monitorDHT(idht *dht.IpfsDHT) {
	for {
		time.Sleep(50 * time.Second)
		peers := idht.RoutingTable().ListPeers()
		log.Printf("DHT routing table contains %d peers", len(peers))
	}
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
