package node

import (
	"bufio"
	"bytes"
	"context"
	go_crypto "crypto"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/handler"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/database"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
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
	"io"
	"log"
	"strings"
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
	ListProviders() (_ map[string][]types.PeerAddrInfo, err error)
	GetOwner() (owner domainGen.Owner, err error)
	AddInfo(ctx context.Context, peerId types.WarpPeerID, info types.NodeInfo) error
	RemoveInfo(ctx context.Context, peerId peer.ID) (err error)
	BlocklistRemove(ctx context.Context, peerId peer.ID) (err error)
	IsBlocklisted(ctx context.Context, peerId peer.ID) (bool, error)
	Blocklist(ctx context.Context, peerId peer.ID) error
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

	ipv4, ipv6 string
}

func createNode(
	ctx context.Context,
	privKey types.WarpPrivateKey,
	store types.WarpPeerstore,
	addrInfos []types.PeerAddrInfo,
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

	relay, err := relayv2.New(node)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		ctx:      ctx,
		node:     node,
		relay:    relay,
		isClosed: new(atomic.Bool),
	}

	n.parseAddresses(node)

	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	log.Printf("NODE STARTED WITH ID %s AND ADDRESSES %s\n", n.ID(), n.node.Addrs())
	fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++")

	return n, nil
}

func NewBootstrapNode(
	ctx context.Context,
	conf config.Config,
) (_ *WarpNode, err error) {
	pk, err := encrypting.GenerateKeyFromSeed([]byte("bootstrap")) // TODO
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

	n, err := createNode(
		ctx,
		pk.(types.WarpPrivateKey),
		store,
		addrInfos,
		conf,
		func(n types.P2PNode) (types.WarpPeerRouting, error) {
			return setupPrivateDHT(ctx, n, mapStore, providersCache, addrInfos)
		},
	)
	if err != nil {
		return nil, err
	}

	n.pubsub, err = NewPubSub(ctx, n.Node(), nil)
	if err != nil {
		return nil, err
	}

	n.mdns = mdns.NewMdnsService(n.node, conf.Node.PSK, NewBootstrapDiscovery(n))
	go func() {
		if err := n.mdns.Start(); err != nil {
			log.Println("mdns failed to start", err)
		}
	}()

	logConf := logging.GetConfig()
	logConf.Labels["node_id"] = n.ID()
	logging.SetupLogging(logConf)

	return n, nil
}

func NewRegularNode(
	ctx context.Context,
	db PersistentLayer,
	conf config.Config,
	timelineRepo *database.TimelineRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	version *semver.Version,
) (_ *WarpNode, err error) {
	//logging.SetLogLevel("*", "INFO")
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

	owner, _ := db.GetOwner()
	discoveryHandler := NewMemberDiscovery(n, userRepo, db)
	n.pubsub, err = NewPubSub(ctx, n.Node(), discoveryHandler)
	if err != nil {
		return nil, err
	}
	n.mdns = mdns.NewMdnsService(n.node, conf.Node.PSK, discoveryHandler)
	go func() {
		if err := n.mdns.Start(); err != nil {
			log.Println("mdns failed to start", err)
			return
		}
		log.Println("mdns service started")
	}()

	n.node.SetStreamHandler("/ping/1.0.0", func(stream network.Stream) {
		defer stream.Close()
		log.Println("received ping")
		stream.Write([]byte("pong"))
	})
	n.node.SetStreamHandler("/timeline/1.0.0", handler.StreamTimelineHandler(timelineRepo))
	n.node.SetStreamHandler("/user/1.0.0", handler.StreamGetUserHandler(userRepo))
	n.node.SetStreamHandler("/tweets/1.0.0", handler.StreamGetTweetsHandler(tweetRepo))
	n.node.SetStreamHandler("/tweet/1.0.0", handler.StreamNewTweetHandler(tweetRepo, timelineRepo))
	n.node.SetStreamHandler("/info/1.0.0", handler.StreamGetInfoHandler(n.Node(), owner, version))
	return n, err
}

const serverNodeAddrDefault = "/ip4/127.0.0.1/tcp/4001/p2p/"

func NewClientNode(ctx context.Context, serverNodeId string, conf config.Config) (_ *WarpNode, err error) {
	if serverNodeId == "" {
		return nil, errors.New("server node ID is empty")
	}
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
		libp2p.UserAgent(conf.Node.PSK+"-client"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client node: %s", err)
	}
	serverAddr := serverNodeAddrDefault + serverNodeId
	maddr, err := ma.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing server address: %s", err)
	}

	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("creating fddress info: %s", err)
	}

	client.Peerstore().AddAddrs(serverInfo.ID, serverInfo.Addrs, peerstore.PermanentAddrTTL)

	n := &WarpNode{
		ctx:      ctx,
		node:     client,
		isClosed: new(atomic.Bool),
	}

	if len(client.Addrs()) != 0 {
		return nil, errors.New("client node must have no addresses")
	}
	response, err := n.StreamSend(
		serverNodeId, "/ping/1.0.0", []byte("ping"),
	)
	log.Printf("response from server: %s\n", response)

	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	log.Println("client node created:", n.node.ID())
	return n, nil
}

func (n *WarpNode) ID() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID().String()
}

func (n *WarpNode) Node() host.Host {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node
}

func (n *WarpNode) Peerstore() types.WarpPeerstore {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node.Peerstore()
}

func (n *WarpNode) parseAddresses(node host.Host) {
	if n == nil {
		return
	}
	for _, a := range node.Addrs() {
		if strings.HasPrefix(a.String(), "/ip4/127.0.0.1") { // localhost is default
			continue
		}
		if strings.HasPrefix(a.String(), "/ip6/::1") { // localhost is default
			continue
		}
		if strings.HasPrefix(a.String(), "/ip4") {
			n.ipv4 = a.String()
		}
		if strings.HasPrefix(a.String(), "/ip6") {
			n.ipv6 = a.String()
		}
	}
}

func (n *WarpNode) IPv4() string {
	return n.ipv4
}

func (n *WarpNode) IPv6() string {
	return n.ipv6
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

func (n *WarpNode) StreamSend(peerID string, path types.WarpDiscriminator, data []byte) ([]byte, error) {
	if n == nil || n.node == nil || peerID == "" || path == "" {
		return nil, errors.New("send: parameters improperly configured")
	}

	ctx, cancelF := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancelF()

	serverAddr := serverNodeAddrDefault + peerID
	maddr, err := ma.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing server address: %s", err)
	}

	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("creating address info: %s", err)
	}
	stream, err := n.node.NewStream(ctx, serverInfo.ID, path)
	if err != nil {
		return nil, fmt.Errorf("opening stream: %s", err)
	}
	defer closeStream(stream)

	var rw = bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	if data != nil {
		fmt.Printf("client sent to %s data with size %d\n", path, len(data))
		_, err = rw.Write(data)
		if err != nil {
			return nil, fmt.Errorf("writing to stream: %s", err)
		}
		flush(rw)
		closeWrite(stream)
	}

	buf := bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(rw)
	if err != nil {
		return nil, fmt.Errorf("reading response: %s", err)
	}
	fmt.Printf("client received response from %s, size %d\n", path, buf.Len())

	return buf.Bytes(), nil
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

func closeWrite(s network.Stream) {
	if err := s.CloseWrite(); err != nil {
		log.Printf("close write: %s", err)
	}
}

// 2025-01-25T19:54:43.327+0400	WARN
//dht	go-libp2p-kad-dht/dht.go:523
//failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
//"error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
//all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
//peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
//but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}

func setupPrivateDHT(
	ctx context.Context,
	n types.P2PNode,
	repo datastore.Batching,
	providerStore providers.ProviderStore,
	relays []types.PeerAddrInfo,
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
		log.Printf("new dht: %v\n", err)
		return nil, err
	}
	go func() {
		if err := DHT.Bootstrap(ctx); err != nil {
			log.Printf("DHT bootstrap: %s", err)
		}
	}()

	if err = <-DHT.RefreshRoutingTable(); err != nil {
		log.Printf("failed to refresh kdht routing table: %v\n", err)
	}

	return DHT, nil
}
