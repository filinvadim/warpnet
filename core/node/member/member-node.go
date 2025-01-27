package member

import (
	"context"
	go_crypto "crypto"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/dht-table"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/p2p"
	warpnet "github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

type PersistentLayer interface {
	warpnet.WarpBatching
	providers.ProviderStore
	GetOwner() (domain.Owner, error)
	SessionToken() string
	PrivateKey() go_crypto.PrivateKey
	ListProviders() (_ map[string][]warpnet.PeerAddrInfo, err error)
	AddInfo(ctx context.Context, peerId warpnet.WarpPeerID, info warpnet.NodeInfo) error
	RemoveInfo(ctx context.Context, peerId warpnet.WarpPeerID) (err error)
	BlocklistRemove(ctx context.Context, peerId warpnet.WarpPeerID) (err error)
	IsBlocklisted(ctx context.Context, peerId warpnet.WarpPeerID) (bool, error)
	Blocklist(ctx context.Context, peerId warpnet.WarpPeerID) error
}

type MDNSServicer interface {
	Start(node *WarpNode)
	Close()
}

type DiscoveryServicer interface {
	HandlePeerFound(warpnet.PeerAddrInfo)
}

type Streamer interface {
	Send(peerAddr *warpnet.PeerAddrInfo, r warpnet.WarpRoute, data []byte) ([]byte, error)
}

type WarpNode struct {
	ctx       context.Context
	node      warpnet.P2PNode
	discovery DiscoveryServicer
	relay     warpnet.WarpRelayCloser
	streamer  Streamer
	isClosed  *atomic.Bool

	ipv4, ipv6 string

	retrier retrier.Retrier
	ownerId string
	version *semver.Version
}

func NewMemberNode(
	ctx context.Context,
	db PersistentLayer,
	conf config.Config,
	discService DiscoveryServicer,
	version *semver.Version,
) (_ *WarpNode, err error) {

	privKey := db.PrivateKey().(warpnet.WarpPrivateKey)
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

	hTable := dht_table.NewDHTable(
		ctx, db, providersCache, bootstrapAddrs,
		discService.HandlePeerFound,
		func(info warpnet.PeerAddrInfo) {
			log.Println("dht: node removed", info.ID)
		},
	)

	n, err := setupMemberNode(ctx, privKey, store, bootstrapAddrs, conf, hTable.Start, version)
	if err != nil {
		return nil, err
	}

	owner, _ := db.GetOwner()
	n.ownerId = owner.UserId

	return n, err
}

func setupMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	store warpnet.WarpPeerstore,
	addrInfos []warpnet.PeerAddrInfo,
	conf config.Config,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
	version *semver.Version,
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

	basichost.DefaultNegotiationTimeout = p2p.DefaultTimeout

	node, err := libp2p.New(
		libp2p.WithDialTimeout(p2p.DefaultTimeout),
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithConnectionTimeout(p2p.DefaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))), // TODO shuffle name. "warpnet" now
		libp2p.UserAgent(p2p.ServiceName),
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
		version:  version,
	}

	n.ipv4, n.ipv6 = parseAddresses(node)

	println()
	fmt.Printf("\033[1mNODE STARTED WITH ID %s AND ADDRESSES %s %s\033[0m\n", n.ID(), n.ipv4, n.ipv6)
	println()

	return n, nil
}

func (n *WarpNode) Connect(p warpnet.PeerAddrInfo) error {
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

func (n *WarpNode) SetStreamHandler(route warpnet.WarpRoute, handler warpnet.WarpStreamHandler) {
	n.node.SetStreamHandler(route.ProtocolID(), handler)
}

func (n *WarpNode) NodeInfo(s warpnet.WarpStream) warpnet.NodeInfo {
	reg := n.Node()
	id := reg.ID()
	addrs := reg.Peerstore().Addrs(id)
	protocols, _ := reg.Peerstore().GetProtocols(id)
	latency := reg.Peerstore().LatencyEWMA(id)
	peerInfo := reg.Peerstore().PeerInfo(id)
	connectedness := reg.Network().Connectedness(id)
	listenAddrs := reg.Network().ListenAddresses()

	plainAddrs := make([]string, 0, len(peerInfo.Addrs))
	for _, a := range peerInfo.Addrs {
		plainAddrs = append(plainAddrs, a.String())
	}

	return warpnet.NodeInfo{
		Addrs:     addrs,
		Protocols: warpnet.FromPrIDToRoutes(protocols),
		Latency:   latency,
		PeerInfo: warpnet.WarpAddrInfo{
			ID:    peerInfo.ID,
			Addrs: plainAddrs,
		},
		NetworkState: connectedness.String(),
		ListenAddrs:  listenAddrs,
		Version:      n.version.String(),
		StreamStats:  s.Stat(),
		OwnerId:      n.ownerId,
	}
}

func (n *WarpNode) ID() warpnet.WarpPeerID {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID()
}

func (n *WarpNode) Node() warpnet.P2PNode {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node
}

func (n *WarpNode) Peerstore() warpnet.WarpPeerstore {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node.Peerstore()
}

func (n *WarpNode) Network() warpnet.WarpNetwork {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node.Network()
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

func (n *WarpNode) GenericStream(nodeId string, path warpnet.WarpRoute, data []byte) ([]byte, error) {
	id, err := warpnet.IDFromBytes([]byte(nodeId))
	if err != nil {
		return nil, err
	}
	return n.streamer.Send(&warpnet.PeerAddrInfo{ID: id}, path, data)
}

func (n *WarpNode) Stop() {
	log.Println("shutting down node...")
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered: %v\n", r)
		}
	}()
	if n.relay != nil {
		_ = n.relay.Close()
	}
	if err := n.node.Close(); err != nil {
		log.Printf("failed to close node: %v", err)
	}
	n.isClosed.Store(true)
	return
}

func parseAddresses(node warpnet.P2PNode) (string, string) {
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
