package node

import (
	"context"
	go_crypto "crypto"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/handler"
	nodeGen "github.com/filinvadim/warpnet/core/node-gen"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/logger"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ws "github.com/libp2p/go-libp2p/p2p/transport/websocket"
	"go.uber.org/zap/zapcore"
	"log"
	"time"
)

const (
	NetworkName    = "warpnet"
	ProtocolPrefix = "/" + NetworkName
)

type PersistentLayer interface {
	datastore.Batching
	providers.ProviderStore
	PrivateKey() go_crypto.PrivateKey
	ListProviders() (_ map[string][]types.WarpAddrInfo, err error)
}

type NodeLogger interface {
	zapcore.Core
	Info(args ...interface{})
}

type WarpNode struct {
	node   types.P2PNode
	mdns   types.WarpMDNS
	relay  types.WarpRelayCloser
	pubsub types.WarpGossiper
}

func createNode(
	ctx context.Context,
	privKey types.WarpPrivateKey,
	store types.WarpPeerstore,
	addrInfos []types.WarpAddrInfo,
	conf config.Config,
	routingFn func(node types.P2PNode) (types.WarpPeerRouting, error),
) (*WarpNode, error) {
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

	node, err := libp2p.New(
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(ws.New),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(NetworkName))), // TODO shuffle name
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
		libp2p.Routing(routingFn),
	)
	if err != nil {
		return nil, err
	}

	mdnsService := mdns.NewMdnsService(node, NetworkName, NewDiscoveryNotifee(node))
	relay, err := relayv2.New(node)
	if err != nil {
		return nil, err
	}

	pubsub, err := NewPubSub(ctx, node)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		node:   node,
		mdns:   mdnsService,
		relay:  relay,
		pubsub: pubsub,
	}

	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	log.Printf("NODE STARTED WITH ID %s AND ADDRESSES %v\n", n.ID(), n.Addresses())
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")

	logConf := logging.GetConfig()
	logConf.Labels["node_id"] = n.ID()
	logging.SetupLogging(logConf)

	return n, nil
}

func NewBootstrapNode(
	ctx context.Context,
	conf config.Config,
	l logger.Core,
) (_ *WarpNode, err error) {
	logging.SetPrimaryCore(l)

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

	addrInfos, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	return createNode(
		ctx, privKey.(crypto.PrivKey), store, addrInfos, conf,
		func(n types.P2PNode) (types.WarpPeerRouting, error) {
			return setupPrivateDHT(ctx, n, mapStore, providersCache, addrInfos)
		},
	)
}

func NewRegularNode(
	ctx context.Context,
	db PersistentLayer,
	conf config.Config,
	l NodeLogger,
) (_ *WarpNode, err error) {
	//core := l.With([]logger.Field{{Key: "node", String: "member"}})
	logging.SetPrimaryCore(l)

	privKey := db.PrivateKey()
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}

	providersCache, err := NewProviderCache(ctx, db)
	if err != nil {
		return nil, err
	}

	addrInfos, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	n, err := createNode(
		ctx, privKey.(crypto.PrivKey), store, addrInfos, conf,
		func(n types.P2PNode) (types.WarpPeerRouting, error) {
			return setupPrivateDHT(ctx, n, db, providersCache, addrInfos)
		},
	)
	if err != nil {
		return nil, err
	}

	sw, _ := nodeGen.GetSwagger()
	h, err := handler.NewNodeStreamHandler(ctx, n.node, l, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	err = h.RegisterHandlers(sw.Paths)
	return n, err
}

func (n *WarpNode) ID() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID().String()
}

func (n *WarpNode) Addresses() (addrs []string) {
	if n == nil || n.node == nil {
		return addrs
	}
	for _, a := range n.node.Addrs() {
		addrs = append(addrs, a.String())
	}
	return addrs
}

func (n *WarpNode) Stop() error {
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
	if n.pubsub != nil {
		_ = n.pubsub.Close()
	}

	return n.node.Close()
}

func setupPrivateDHT(
	ctx context.Context,
	n types.P2PNode,
	repo datastore.Batching,
	providerStore providers.ProviderStore,
	relays []types.WarpAddrInfo,
) (*types.WarpDHT, error) {
	DHT, err := dht.New(
		ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(ProtocolPrefix),
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

func sendMessage(h types.P2PNode, peerID types.WarpPeerID, protocol types.WarpDiscriminator, message string) {
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
