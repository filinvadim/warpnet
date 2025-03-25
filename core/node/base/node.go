package base

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/relay"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/security"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/pnet"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

type Streamer interface {
	Send(peerAddr warpnet.PeerAddrInfo, r stream.WarpRoute, data []byte) ([]byte, error)
}

const (
	DefaultTimeout = 180 * time.Second
	ServiceName    = "warpnet"
)

type WarpNode struct {
	ctx      context.Context
	node     warpnet.P2PNode
	relay    warpnet.WarpRelayCloser
	streamer Streamer

	ipv4, ipv6, ownerId string
	isClosed            *atomic.Bool
	version             *semver.Version

	nodeInfo warpnet.NodeInfo
}

func NewWarpNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	store warpnet.WarpPeerstore,
	ownerId string,
	psk security.PSK,
	listenAddr string,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
) (*WarpNode, error) {
	limiter := rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale())

	manager, err := connmgr.NewConnManager(
		100,
		limiter.GetConnLimits().GetConnTotalLimit(),
		connmgr.WithGracePeriod(time.Hour*12),
	)
	if err != nil {
		return nil, err
	}

	peersList := store.PeersWithAddrs()
	staticRelaysList := make([]warpnet.PeerAddrInfo, 0, len(peersList))
	for _, p := range peersList {
		info := store.PeerInfo(p)
		staticRelaysList = append(staticRelaysList, info)
	}

	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	node, err := libp2p.New(
		libp2p.WithDialTimeout(DefaultTimeout),
		libp2p.ListenAddrStrings(
			listenAddr,
		),
		libp2p.SwarmOpts(
			swarm.WithDialTimeout(DefaultTimeout),
			swarm.WithDialTimeoutLocal(DefaultTimeout),
		),
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithConnectionTimeout(DefaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(pnet.PSK(psk)),
		libp2p.UserAgent(ServiceName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableRelay(),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.EnableAutoRelayWithStaticRelays(staticRelaysList),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}

	nodeRelay, err := relay.NewRelay(node)
	if err != nil {
		return nil, err
	}

	ipv4, ipv6 := parseAddresses(node)
	id := node.ID()
	latency := node.Peerstore().LatencyEWMA(id)
	peerInfo := node.Peerstore().PeerInfo(id)
	connectedness := node.Network().Connectedness(id)

	plainAddrs := make([]string, 0, len(peerInfo.Addrs))
	for _, a := range peerInfo.Addrs {
		plainAddrs = append(plainAddrs, a.String())
	}

	wn := &WarpNode{
		ctx:      ctx,
		node:     node,
		relay:    nodeRelay,
		ipv6:     ipv6,
		ipv4:     ipv4,
		ownerId:  ownerId,
		streamer: stream.NewStreamPool(ctx, node),
		isClosed: new(atomic.Bool),
		version:  config.ConfigFile.Version,
	}

	nodeInfo := warpnet.NodeInfo{
		ID:           node.ID(),
		Addrs:        []string{ipv4, ipv6},
		Protocols:    wn.SupportedProtocols(),
		Latency:      latency,
		NetworkState: connectedness.String(),
		Version:      wn.version,
		//StreamStats:  nil, // will be added later
		OwnerId: ownerId,
		PSK:     pnet.PSK(psk),
	}
	wn.nodeInfo = nodeInfo

	return wn, nil
}

func (n *WarpNode) Connect(p warpnet.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}

	if n.node.Network().Connectedness(p.ID) == network.Connected {
		return nil
	}

	log.Infoln("connect attempt to node:", p.ID.String(), p.Addrs)
	if err := n.node.Connect(n.ctx, p); err != nil {
		return fmt.Errorf("failed to connect to node: %w", err)
	}
	log.Infoln("connect attempt successful:", p.ID.String())

	return nil
}

func (n *WarpNode) SetStreamHandler(route stream.WarpRoute, handler warpnet.WarpStreamHandler) {
	if !stream.IsValidRoute(route) {
		log.Fatalf("invalid route: %v", route)
	}
	n.node.SetStreamHandler(route.ProtocolID(), handler)
}

func (n *WarpNode) SupportedProtocols() []string {
	protocols := n.node.Mux().Protocols()
	filtered := make([]string, 0, len(protocols))
	for _, p := range protocols {
		if strings.HasPrefix(string(p), "/ipfs") {
			continue
		}
		if strings.HasPrefix(string(p), "/libp2p") {
			continue
		}
		if strings.Contains(string(p), "/kad/") {
			continue
		}
		if strings.HasPrefix(string(p), "/meshsub") {
			continue
		}
		if strings.HasPrefix(string(p), "/floodsub") {
			continue
		}
		if strings.HasPrefix(string(p), "/private") { // hide it just in case
			continue
		}
		filtered = append(filtered, string(p))
	}
	return filtered
}

func (n *WarpNode) NodeInfo() warpnet.NodeInfo {
	return n.nodeInfo
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

func (n *WarpNode) Mux() warpnet.WarpProtocolSwitch {
	return n.node.Mux()
}

type streamNodeID = string

func (n *WarpNode) GenericStream(nodeIdStr streamNodeID, path stream.WarpRoute, data any) (_ []byte, err error) {
	if n == nil || n.streamer == nil {
		return nil, errors.New("node is not initialized")
	}

	nodeId := warpnet.FromStringToPeerID(nodeIdStr)
	if nodeId == "" {
		return nil, fmt.Errorf("invalid node id: %v", nodeIdStr)
	}
	if n.NodeInfo().ID == nodeId {
		return nil, nil // self request discarded
	}

	peerInfo := n.Peerstore().PeerInfo(nodeId)
	if len(peerInfo.Addrs) == 0 {
		log.Errorf("peer %v does not have any addresses: %v", nodeId, peerInfo.Addrs)
		return nil, warpnet.ErrNodeIsOffline
	}
	for _, addr := range peerInfo.Addrs {
		log.Infof("new node is dialable: %s %t\n", addr.String(), n.Network().CanDial(peerInfo.ID, addr))
	}

	var bt []byte
	if data != nil {
		var ok bool
		bt, ok = data.([]byte)
		if !ok {
			bt, err = json.JSON.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("generic stream: marshal data %v %s", err, data)
			}
		}
	}

	return n.streamer.Send(peerInfo, path, bt)
}

func (n *WarpNode) StopNode() {
	log.Infoln("shutting down node...")
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered: %v\n", r)
		}
	}()
	if n == nil || n.node == nil {
		return
	}
	if n.relay != nil {
		_ = n.relay.Close()
	}
	if err := n.node.Close(); err != nil {
		log.Errorf("failed to close node: %v", err)
	}
	n.isClosed.Store(true)
	n.node = nil
	time.Sleep(time.Duration(rand.Intn(5)) * time.Second) // jitter

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
