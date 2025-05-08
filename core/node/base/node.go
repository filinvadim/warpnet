package base

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/security"
	golog "github.com/ipfs/go-log/v2"
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

/*
  The libp2p Relay v2 library is an improved version of the connection relay mechanism in libp2p,
  designed for scenarios where nodes cannot establish direct connections due to NAT, firewalls,
  or other network restrictions.

  In a standard P2P network, nodes are expected to connect directly to each other. However, if one
  or both nodes are behind NAT or a firewall, direct connections may be impossible. In such cases,
  Relay v2 enables traffic to be relayed through intermediary nodes (relay nodes), allowing communication
  even in restricted network environments.

  ### **Key Features of libp2p Relay v2:**
  - **Automatic Relay Discovery and Usage**
    - If a node cannot establish a direct connection, it automatically finds a relay node.
  - **Operating Modes:**
    - **Active relay:** A node can act as a relay for others.
    - **Passive relay:** A node uses relay services without providing its own resources.
  - **Hole Punching**
    - Uses NAT traversal techniques (e.g., **DCUtR – Direct Connection Upgrade through Relay**)
      to attempt a direct connection before falling back to a relay.
  - **More Efficient Routing**
    - Relay v2 selects low-latency routes instead of simply forwarding all traffic through a single node.
  - **Bandwidth Optimization**

  ### **When is Relay v2 Needed?**
  - Nodes do not have a public IP address and are behind NAT.
  - A connection is required between network participants who cannot connect directly.
  - Reducing relay server load is important (compared to Relay v1).
  - **DCUtR is used** to attempt NAT traversal before falling back to a relay.
*/

const (
	DefaultRelayDataLimit     = 32 << 20 // 32 MiB
	DefaultRelayDurationLimit = 5 * time.Minute

	DefaultTimeout = 360 * time.Second
	ServiceName    = "warpnet"
)

type Streamer interface {
	Send(peerAddr warpnet.PeerAddrInfo, r stream.WarpRoute, data []byte) ([]byte, error)
}

type AddressManager interface {
	Add(addr warpnet.WarpAddress, ttl time.Duration)
}

type WarpNode struct {
	ctx         context.Context
	node        warpnet.P2PNode
	addrManager AddressManager

	streamer Streamer

	ownerId  string
	isClosed *atomic.Bool
	version  *semver.Version

	startTime time.Time
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
	defaultLimits := rcmgr.DefaultLimits.AutoScale()
	limiter := rcmgr.NewFixedLimiter(defaultLimits)

	manager, err := connmgr.NewConnManager(
		100,
		limiter.GetConnLimits().GetConnTotalLimit(),
		connmgr.WithGracePeriod(time.Hour*12),
	)
	if err != nil {
		return nil, err
	}

	infos, err := config.ConfigFile.Node.AddrInfos()
	if err != nil {
		return nil, err
	}
	staticRelaysList := make([]warpnet.PeerAddrInfo, 0, len(infos))
	for _, info := range infos {
		staticRelaysList = append(staticRelaysList, info)
	}

	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	addrManager := NewAddressManager()

	_ = golog.SetLogLevel("autorelay", "DEBUG")

	node, err := libp2p.New(
		libp2p.WithDialTimeout(DefaultTimeout),
		libp2p.ListenAddrStrings(
			listenAddr,
		),
		libp2p.AddrsFactory(addrManager.Factory()),
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
		libp2p.PrivateNetwork(pnet.PSK(psk)),
		libp2p.UserAgent(ServiceName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithLimit(&relayv2.RelayLimit{
			Duration: DefaultRelayDurationLimit,
			Data:     DefaultRelayDataLimit,
		})),

		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(staticRelaysList),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),
		libp2p.ForceReachabilityPrivate(),
	)
	if err != nil {
		return nil, fmt.Errorf("node: failed to init node: %v", err)
	}

	id := node.ID()
	peerInfo := node.Peerstore().PeerInfo(id)

	plainAddrs := make([]string, 0, len(peerInfo.Addrs))
	for _, a := range peerInfo.Addrs {
		plainAddrs = append(plainAddrs, a.String())
	}

	wn := &WarpNode{
		ctx:         ctx,
		node:        node,
		addrManager: addrManager,
		ownerId:     ownerId,
		streamer:    stream.NewStreamPool(ctx, node),
		isClosed:    new(atomic.Bool),
		version:     config.ConfigFile.Version,
		startTime:   time.Now(),
	}

	return wn, nil
}

func (n *WarpNode) Connect(p warpnet.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}

	peerState := n.node.Network().Connectedness(p.ID)
	isConnected := peerState == network.Connected || peerState == network.Limited
	if isConnected {
		return nil
	}

	log.Infoln("node: connect attempt to node:", p.ID.String(), p.Addrs)
	if err := n.node.Connect(n.ctx, p); err != nil {
		return fmt.Errorf("failed to connect to node: %w", err)
	}
	log.Infoln("node: connect attempt successful:", p.ID.String())

	return nil
}

func (n *WarpNode) SetStreamHandler(route stream.WarpRoute, handler warpnet.WarpStreamHandler) {
	if !stream.IsValidRoute(route) {
		log.Fatalf("node: invalid route: %v", route)
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
		if strings.Contains(string(p), "/admin/") {
			continue
		}
		if strings.HasPrefix(string(p), "/raft") {
			continue
		}
		filtered = append(filtered, string(p))
	}
	return filtered
}

func (n *WarpNode) NodeInfo() warpnet.NodeInfo {
	if n == nil || n.node == nil || n.node.Network() == nil || n.node.Peerstore() == nil {
		return warpnet.NodeInfo{}
	}

	addrs := n.node.Peerstore().Addrs(n.node.ID())
	addresses := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if !warpnet.IsPublicMultiAddress(a) {
			continue
		}
		if strings.Contains(a.String(), "p2p-circuit") {
			continue
		}
		addresses = append(addresses, a.String())
	}

	relayState := "Waiting..."
	for _, addr := range n.node.Addrs() {
		if strings.Contains(addr.String(), "p2p-circuit") {
			relayState = "Running"
		}
	}

	return warpnet.NodeInfo{
		ID:         n.node.ID(),
		Addresses:  addresses,
		Version:    n.version,
		OwnerId:    n.ownerId,
		StartTime:  n.startTime,
		RelayState: relayState,
	}
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

var ErrSelfRequest = errors.New("self request")

func (n *WarpNode) Stream(nodeIdStr string, path stream.WarpRoute, data any) (_ []byte, err error) {
	if n == nil || n.streamer == nil {
		return nil, errors.New("node is not initialized")
	}
	if nodeIdStr == "" {
		return nil, errors.New("node: empty node id")
	}
	nodeId := warpnet.FromStringToPeerID(nodeIdStr)
	if nodeId == "" {
		return nil, fmt.Errorf("node: invalid node id: %v", nodeIdStr)
	}
	if n.NodeInfo().ID == nodeId {
		return nil, ErrSelfRequest
	}

	peerInfo := n.Peerstore().PeerInfo(nodeId)
	if len(peerInfo.Addrs) == 0 {
		log.Warningf("node %v is offline", nodeId)
		return nil, warpnet.ErrNodeIsOffline
	}

	var bt []byte
	if data != nil {
		var ok bool
		bt, ok = data.([]byte)
		if !ok {
			bt, err = json.JSON.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("node: generic stream: marshal data %v %s", err, data)
			}
		}
	}
	return n.streamer.Send(peerInfo, path, bt)
}

func (n *WarpNode) AddOwnPublicAddress(remoteAddr string) error {
	if remoteAddr == "" {
		return errors.New("node: empty node info public address")
	}
	maddr, err := warpnet.NewMultiaddr(remoteAddr)
	if err != nil {
		return err
	}

	week := time.Hour * 24 * 7
	n.Peerstore().AddAddr(n.node.ID(), maddr, week)
	n.addrManager.Add(maddr, week)
	return nil
}

func (n *WarpNode) StopNode() {
	log.Infoln("node: shutting down node...")
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("node: recovered: %v\n", r)
		}
	}()
	if n == nil || n.node == nil {
		return
	}

	if err := n.node.Close(); err != nil {
		log.Errorf("node: failed to close: %v", err)
	}
	n.isClosed.Store(true)
	n.node = nil
	n.addrManager = nil
	time.Sleep(time.Duration(rand.Intn(999)) * time.Millisecond) // jitter

	return
}
