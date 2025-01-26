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
	"github.com/filinvadim/warpnet/retrier"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"

	//"github.com/filinvadim/warpnet/database"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"io"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

const (
	ServiceName    = "warpnet"
	ProtocolPrefix = "/warpnet"
	defaultTimeout = 60 * time.Second
)

type PersistentLayer interface {
	types.WarpBatching
	providers.ProviderStore
	PrivateKey() go_crypto.PrivateKey
	ListProviders() (_ map[string][]types.PeerAddrInfo, err error)
	GetOwner() (owner domainGen.Owner, err error)
	AddInfo(ctx context.Context, peerId types.WarpPeerID, info types.NodeInfo) error
	RemoveInfo(ctx context.Context, peerId types.WarpPeerID) (err error)
	BlocklistRemove(ctx context.Context, peerId types.WarpPeerID) (err error)
	IsBlocklisted(ctx context.Context, peerId types.WarpPeerID) (bool, error)
	Blocklist(ctx context.Context, peerId types.WarpPeerID) error
}

type MDNSServicer interface {
	Start(node *WarpNode)
	Close()
}

type DiscoveryServicer interface {
	Run(n DiscoveryInfoStorer)
	Close()
}

type WarpNode struct {
	ctx       context.Context
	node      types.P2PNode
	mdns      MDNSServicer
	discovery DiscoveryServicer
	relay     types.WarpRelayCloser
	pubsub    types.WarpGossiper
	isClosed  *atomic.Bool

	ipv4, ipv6   string
	clientPeerID types.WarpPeerID

	retrier retrier.Retrier
}

func setupNode(
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
		connmgr.WithGracePeriod(time.Hour),
	)
	if err != nil {
		return nil, err
	}

	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	basichost.DefaultNegotiationTimeout = defaultTimeout

	node, err := libp2p.New(
		libp2p.WithDialTimeout(defaultTimeout),
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithConnectionTimeout(defaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))), // TODO shuffle name. "warpnet" now
		libp2p.UserAgent(ServiceName),
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

	relay, err := relayv2.New(
		node,
		relayv2.WithResources(relayv2.DefaultResources()),
		relayv2.WithInfiniteLimits(),
	)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		ctx:      ctx,
		node:     node,
		relay:    relay,
		isClosed: new(atomic.Bool),
		retrier:  retrier.New(time.Second * 5),
	}

	n.ipv4, n.ipv6 = n.parseAddresses(node)

	println()
	fmt.Printf("\033[1mNODE STARTED WITH ID %s AND ADDRESSES %s %s\033[0m\n", n.ID(), n.ipv4, n.ipv6)
	println()

	return n, nil
}

func NewBootstrapNode(
	ctx context.Context,
	conf config.Config,
) (_ *WarpNode, err error) {
	privKey, err := encrypting.GenerateKeyFromSeed([]byte("bootstrap")) // TODO
	if err != nil {
		return nil, err
	}

	warpPrivKey := privKey.(types.WarpPrivateKey)
	id, err := types.IDFromPrivateKey(warpPrivKey)
	if err != nil {
		return nil, err
	}

	store, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	mapStore := datastore.NewMapDatastore()
	providersCache, err := providers.NewProviderManager(id, store, mapStore)
	if err != nil {
		return nil, err
	}

	bootstrapAddrs, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	hTable := NewDHTable(
		ctx, mapStore, providersCache, bootstrapAddrs,
		func(info types.PeerAddrInfo) {
			log.Println("dht: node added", info.ID)
		},
		func(info types.PeerAddrInfo) {
			log.Println("dht: node removed", info.ID)
		},
	)

	n, err := setupNode(
		ctx,
		warpPrivKey,
		store,
		bootstrapAddrs,
		conf,
		hTable.Start,
	)
	if err != nil {
		return nil, err
	}

	mdns := NewMulticastDNS(ctx, nil)
	go mdns.Start(n)
	n.mdns = mdns

	pubsub, err := NewPubSub(ctx, n, nil)
	if err != nil {
		return nil, err
	}
	n.pubsub = pubsub
	go pubsub.RunDiscovery()

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
	privKey := db.PrivateKey().(types.WarpPrivateKey)
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}

	providersCache, err := NewProviderCache(ctx, db)
	if err != nil {
		return nil, err
	}

	bootstrapAddrs, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	discService := NewDiscoveryService(ctx, userRepo, db)

	hTable := NewDHTable(
		ctx, db, providersCache, bootstrapAddrs,
		discService.HandlePeerFound,
		func(info types.PeerAddrInfo) {
			log.Println("dht: node removed", info.ID)
		},
	)

	mdns := NewMulticastDNS(ctx, discService.HandlePeerFound)

	n, err := setupNode(ctx, privKey, store, bootstrapAddrs, conf, hTable.Start)
	if err != nil {
		return nil, err
	}

	go discService.Run(n)
	n.discovery = discService

	go mdns.Start(n)
	n.mdns = mdns

	pubsub, err := NewPubSub(ctx, n, discService.HandlePeerFound)
	if err != nil {
		return nil, err
	}
	go pubsub.RunDiscovery()
	n.pubsub = pubsub

	n.node.SetStreamHandler("/ping/1.0.0", n.Pong)
	n.node.SetStreamHandler("/timeline/1.0.0", handler.StreamTimelineHandler(timelineRepo))
	n.node.SetStreamHandler("/user/1.0.0", handler.StreamGetUserHandler(userRepo))
	n.node.SetStreamHandler("/tweets/1.0.0", handler.StreamGetTweetsHandler(tweetRepo))
	n.node.SetStreamHandler("/tweet/1.0.0", handler.StreamNewTweetHandler(tweetRepo, timelineRepo))
	n.node.SetStreamHandler("/info/1.0.0", handler.StreamGetInfoHandler(n.Node(), db, version))
	return n, err
}

const serverNodeAddrDefault = "/ip4/127.0.0.1/tcp/4001/p2p/"

func NewClientNode(ctx context.Context, serverNodeId string, conf config.Config) (_ *WarpNode, err error) {
	if serverNodeId == "" {
		return nil, errors.New("client node: server node ID is empty")
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
		return nil, fmt.Errorf("client node: creating %s", err)
	}
	serverAddr := serverNodeAddrDefault + serverNodeId
	maddr, err := types.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("client node: parsing server address: %s", err)
	}

	serverInfo, err := types.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("client node: creating address info: %s", err)
	}

	client.Peerstore().AddAddrs(serverInfo.ID, serverInfo.Addrs, types.PermanentAddrTTL)

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
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	log.Printf("client-server nodes ping-%s complete\n", response)

	log.Println("client node created:", n.node.ID())
	return n, nil
}

func (n *WarpNode) Pong(stream types.WarpStream) {
	n.clientPeerID = stream.Conn().RemotePeer() // TODO
	_, _ = stream.Write([]byte("pong"))
	_ = stream.Close()
}

func (n *WarpNode) Connect(p types.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}
	now := time.Now()
	err := n.retrier.Try(
		func() (bool, error) {
			if err := n.node.Connect(n.ctx, p); err != nil {
				log.Println("node connect error:", err)
				return false, nil
			}
			return true, nil
		},
		now.Add(time.Minute*2),
	)
	return err
}

func (n *WarpNode) ID() types.WarpPeerID {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID()
}

func (n *WarpNode) Node() types.P2PNode {
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

func (n *WarpNode) parseAddresses(node types.P2PNode) (string, string) {
	if n == nil {
		return "", ""
	}
	var (
		ipv4, ipv6 string
	)
	for _, a := range node.Addrs() {
		if strings.HasPrefix(a.String(), "/ip4/127.0.0.1") { // localhost is default
			continue
		}
		if strings.HasPrefix(a.String(), "/ip6/::1") { // localhost is default
			continue
		}
		if strings.HasPrefix(a.String(), "/ip4") {
			ipv4 = a.String()
		}
		if strings.HasPrefix(a.String(), "/ip6") {
			ipv6 = a.String()
		}
	}
	return ipv4, ipv6
}
func (n *WarpNode) Addrs() []string {
	return []string{n.ipv4, n.ipv6}
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
		n.mdns.Close()
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
	if n == nil {
		return nil, nil
	}

	serverAddr := serverNodeAddrDefault + peerID
	maddr, err := types.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("stream: parsing server address: %s", err)
	}

	serverInfo, err := types.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("stream: creating address info: %s", err)
	}

	return send(n.node, serverInfo, path, data)
}

func send(n types.P2PNode, serverInfo *types.PeerAddrInfo, path types.WarpDiscriminator, data []byte) ([]byte, error) {
	if n == nil || serverInfo == nil || path == "" {
		return nil, errors.New("stream: parameters improperly configured")
	}

	stream, err := n.NewStream(context.Background(), serverInfo.ID, path)
	if err != nil {
		return nil, fmt.Errorf("stream: opening: %s", err)
	}
	defer closeStream(stream)

	var rw = bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	if data != nil {
		log.Printf("stream: sent to %s data with size %d\n", path, len(data))
		_, err = rw.Write(data)
		flush(rw)
		closeWrite(stream)
		if err != nil {
			return nil, fmt.Errorf("stream: writing: %s", err)
		}
	}

	buf := bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(rw)
	if err != nil {
		return nil, fmt.Errorf("reading response: %s", err)
	}
	log.Printf("stream: received response from %s, size %d\n", path, buf.Len())

	return buf.Bytes(), nil
}

func closeStream(stream types.WarpStream) {
	if err := stream.Close(); err != nil {
		log.Printf("closing stream: %s", err)
	}
}

func flush(rw *bufio.ReadWriter) {
	if err := rw.Flush(); err != nil {
		log.Printf("flush: %s", err)
	}
}

func closeWrite(s types.WarpStream) {
	if err := s.CloseWrite(); err != nil {
		log.Printf("close write: %s", err)
	}
}
