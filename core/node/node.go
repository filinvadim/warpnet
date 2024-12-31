package node

import (
	"context"
	go_crypto "crypto"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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

type PersistentLayer interface {
	datastore.Batching
	providers.ProviderStore
	PrivateKey() go_crypto.PrivateKey
	ListProviders() (_ map[string][]peer.AddrInfo, err error)
}

type Node struct {
	db PersistentLayer

	node   host.Host
	mdns   mdns.Service
	relay  *relayv2.Relay
	pubsub *Gossip
}

func NewBootstrapNode(ctx context.Context, conf config.Config) (_ *Node, err error) {
	logging.SetLogLevel("*", conf.Node.Logging.Level)

	privKey, err := encrypting.GenerateKeyFromSeed([]byte(conf.Node.SeedID))
	if err != nil {
		return nil, err
	}
	store, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	mapStore := datastore.NewMapDatastore()
	providersCache, err := providers.NewProviderManager("bootstrap", store, mapStore)
	if err != nil {
		return nil, err
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

	addrInfos, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(ws.New),
		libp2p.Identity(privKey.(crypto.PrivKey)),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(NetworkName))),
		libp2p.UserAgent(NetworkName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(addrInfos),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			return setupPrivateDHT(ctx, h, mapStore, providersCache, addrInfos)
		}),
	)
	if err != nil {
		return nil, err
	}
	mdnsService := mdns.NewMdnsService(node, NetworkName, NewDiscoveryNotifee(node))
	relay, err := relayv2.New(node)
	if err != nil {
		return nil, err
	}

	n := &Node{
		node:  node,
		mdns:  mdnsService,
		relay: relay,
	}
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	log.Printf("BOOTSRAP NODE STARTED WITH ID %s AND ADDRESSES %v\n", n.ID(), n.Addresses())
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")

	logConf := logging.GetConfig()
	logConf.Labels["node_id"] = n.ID()
	logging.SetupLogging(logConf)

	pubsub, err := NewPubSub(ctx, n.node)
	if err != nil {
		return nil, err
	}
	n.pubsub = pubsub
	return n, nil
}

func NewRegularNode(
	ctx context.Context,
	db PersistentLayer,
	conf config.Config,
) (_ *Node, err error) {
	logging.SetLogLevel("*", conf.Node.Logging.Level)

	privKey := db.PrivateKey()
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}
	providersCache, err := NewProviderCache(ctx, db)
	if err != nil {
		return nil, err
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

	addrInfos, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(ws.New),
		libp2p.Identity(privKey.(crypto.PrivKey)),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(NetworkName))),
		libp2p.UserAgent(NetworkName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(addrInfos),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			return setupPrivateDHT(ctx, h, db, providersCache, addrInfos)
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

	pubsub, err := NewPubSub(ctx, node)
	if err != nil {
		return nil, err
	}

	n := &Node{
		db,
		node,
		mdnsService,
		relay,
		pubsub,
	}

	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	log.Printf("REGULAR NODE STARTED WITH ID %s AND ADDRESSES %v\n", n.ID(), n.Addresses())
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	logConf := logging.GetConfig()
	logConf.Labels["node_id"] = n.ID()
	logging.SetupLogging(logConf)
	return n, nil
}

func (n *Node) ID() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID().String()
}

func (n *Node) Addresses() (addrs []string) {
	if n == nil || n.node == nil {
		return addrs
	}
	for _, a := range n.node.Addrs() {
		addrs = append(addrs, a.String())
	}
	return addrs
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
	if n.db != nil {
		_ = n.db.Close()
	}
	if n.pubsub != nil {
		_ = n.pubsub.Close()
	}

	return n.node.Close()
}

func setupPrivateDHT(
	ctx context.Context,
	h host.Host,
	repo datastore.Batching,
	providerStore providers.ProviderStore,
	relays []peer.AddrInfo,
) (*dht.IpfsDHT, error) {
	DHT, err := dht.New(
		ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/"+NetworkName),
		dht.Datastore(repo),
		dht.BootstrapPeers(relays...),
		dht.ProviderStore(providerStore),
	)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := DHT.Bootstrap(ctx); err != nil {
			log.Printf("failed to bootstrap dht: %v\n", err)
		}
	}()
	if err = <-DHT.RefreshRoutingTable(); err != nil {
		log.Printf("failed to refresh kdht routing table: %v\n", err)
	}
	return DHT, nil
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
