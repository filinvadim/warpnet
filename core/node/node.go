package node

import (
	"bufio"
	"bytes"
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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap/zapcore"
	"log"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	ProtocolPrefix = "/warpnet"
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
	ctx      context.Context
	node     types.P2PNode
	mdns     types.WarpMDNS
	relay    types.WarpRelayCloser
	pubsub   types.WarpGossiper
	isClosed *atomic.Bool
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
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))), // TODO shuffle name. "warpnet" now
		libp2p.UserAgent(conf.Node.PSK),
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

	mdnsService := mdns.NewMdnsService(node, conf.Node.PSK, NewDiscoveryNotifee(node))
	relay, err := relayv2.New(node)
	if err != nil {
		return nil, err
	}

	pubsub, err := NewPubSub(ctx, node)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		ctx:      ctx,
		node:     node,
		mdns:     mdnsService,
		relay:    relay,
		pubsub:   pubsub,
		isClosed: new(atomic.Bool),
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
	seedId := []byte(strconv.Itoa(conf.Node.SeedID))
	privKey, err := encrypting.GenerateKeyFromSeed(seedId)
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

const serverNodeAddr = "/ip4/127.0.0.1/tcp/4001/p2p/"

func NewClientNode(ctx context.Context, serverNodeId string, conf config.Config) (_ *WarpNode, err error) {
	libp2p.DefaultMuxers = nil
	client, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.DisableMetrics(),
		libp2p.DisableRelay(),
		libp2p.RandomIdentity,
		libp2p.Ping(false),
		libp2p.ForceReachabilityPrivate(),
		libp2p.DisableIdentifyAddressDiscovery(),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))),
		libp2p.UserAgent(conf.Node.PSK),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client node: %s", err)
	}
	serverAddr := serverNodeAddr + serverNodeId
	maddr, err := ma.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing server address: %s", err)
	}

	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("creating Address info: %s", err)
	}

	if err := client.Connect(context.Background(), *serverInfo); err != nil {
		return nil, fmt.Errorf("connecting to server node %s: %v", serverInfo.ID, err)
	}
	n := &WarpNode{
		ctx:      ctx,
		node:     client,
		isClosed: new(atomic.Bool),
	}
	return n, nil
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

func (n *WarpNode) Stop() {
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
	if err := n.node.Close(); err != nil {
		log.Printf("failed to close node: %v", err)
	}
	n.isClosed.Store(true)
	return
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
	go DHT.Bootstrap(ctx)

	if err = <-DHT.RefreshRoutingTable(); err != nil {
		log.Printf("failed to refresh kdht routing table: %v\n", err)
	}
	return DHT, nil
}

// path = "/example/1.0.0"
func (n *WarpNode) StreamSend(peerID types.WarpPeerID, path types.WarpDiscriminator, data []byte) ([]byte, error) {
	stream, err := n.node.NewStream(n.ctx, peerID, path)
	if err != nil {
		return nil, fmt.Errorf("opening stream: %s", err)
	}
	defer closeStream(stream)

	var rw = bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, []byte("\n")...)
	}
	_, err = rw.Write(data)
	if err != nil {
		return nil, fmt.Errorf("writing to stream: %s", err)
	}
	defer flush(rw)

	response, err := rw.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("reading response: %s", err)
	}
	fmt.Printf("Response from server: %s\n", response)
	return response, nil
}

func closeStream(stream network.Stream) {
	if err := stream.Close(); err != nil {
		log.Printf("closing stream: %s", err)
	}
}

func flush(rw *bufio.ReadWriter) {
	if err := rw.Flush(); err != nil {
		log.Printf("flush: %s", err)
	}
}
