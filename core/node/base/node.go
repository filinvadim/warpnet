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
	golog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	libp2pConfig "github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync/atomic"
	"time"
)

var _ = golog.Config{}

const (
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
	relay       *relayv2.Relay
	streamer    Streamer

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

	currentNodeID, _ := warpnet.IDFromPrivateKey(privKey)
	staticRelaysList := make([]warpnet.PeerAddrInfo, 0, len(infos))
	for _, info := range infos {
		if info.ID == currentNodeID {
			continue
		}
		staticRelaysList = append(staticRelaysList, info)
	}

	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	addrManager := NewAddressManager()

	_ = golog.SetLogLevel("autorelay", "DEBUG")
	_ = golog.SetLogLevel("autonatv2", "DEBUG")
	_ = golog.SetLogLevel("p2p-holepunch", "DEBUG")
	_ = golog.SetLogLevel("identify", "DEBUG")

	reachibilityF := libp2p.ForceReachabilityPrivate
	autotaticRelaysF := libp2p.EnableAutoRelayWithStaticRelays
	natServiceF := func() libp2p.Option { return disabled() }
	natPortMapF := libp2p.NATPortMap
	holePunchingF := libp2p.EnableHolePunching
	if ownerId == warpnet.BootstrapOwner {
		reachibilityF = libp2p.ForceReachabilityPublic
		autotaticRelaysF = func(static []peer.AddrInfo, opts ...autorelay.Option) libp2p.Option {
			return disabled()
		}
		natServiceF = libp2p.EnableNATService
		natPortMapF = disabled
		holePunchingF = func(opts ...holepunch.Option) libp2p.Option {
			return func(cfg *libp2pConfig.Config) error {
				return nil
			}
		}

		_ = golog.SetLogLevel("relay", "DEBUG")
		_ = golog.SetLogLevel("nat", "DEBUG")
		_ = golog.SetLogLevel("p2p-circuit", "DEBUG")

	}

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
		natServiceF(),
		natPortMapF(),
		libp2p.PrivateNetwork(pnet.PSK(psk)),
		libp2p.UserAgent(ServiceName),
		holePunchingF(),
		libp2p.Peerstore(store),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelay(),
		libp2p.EnableRelayService(relayv2.WithResources(relay.DefaultResources)),
		autotaticRelaysF(staticRelaysList),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),
		reachibilityF(),
	)
	if err != nil {
		return nil, fmt.Errorf("node: failed to init node: %v", err)
	}

	relayService, err := relay.NewRelay(node)
	if err != nil {
		return nil, fmt.Errorf("node: failed to create relay: %v", err)
	}

	wn := &WarpNode{
		ctx:         ctx,
		node:        node,
		addrManager: addrManager,
		relay:       relayService,
		ownerId:     ownerId,
		streamer:    stream.NewStreamPool(ctx, node),
		isClosed:    new(atomic.Bool),
		version:     config.ConfigFile.Version,
		startTime:   time.Now(),
	}

	return wn, wn.validateSupportedProtocols()
}

func disabled() libp2p.Option {
	return func(cfg *libp2pConfig.Config) error { return nil }
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

func (n *WarpNode) validateSupportedProtocols() error {
	protocols := n.node.Mux().Protocols()
	log.Infoln("node: supported protocols:", protocols)
	var (
		isAutoNatBackFound, isAutoNatRequestFound, isRelayHopFound, isRelayStopFound bool
	)

	for _, proto := range protocols {
		if strings.Contains(string(proto), "autonat/2/dial-back") {
			isAutoNatBackFound = true
		}
		if strings.Contains(string(proto), "autonat/2/dial-request") {
			isAutoNatRequestFound = true
		}
		if strings.Contains(string(proto), "relay/0.2.0/hop") {
			isRelayHopFound = true
		}
		if strings.Contains(string(proto), "relay/0.2.0/stop") {
			isRelayStopFound = true
		}
	}
	if isAutoNatBackFound && isAutoNatRequestFound && isRelayHopFound && isRelayStopFound {
		return nil
	}
	return fmt.Errorf(
		"node: not all supported protocols: autonat/dial-back=%t, autonat/dial-request=%t, relay/hop=%t, relay/stop=%t",
		isAutoNatBackFound, isAutoNatRequestFound, isRelayHopFound, isRelayStopFound,
	)
}

func (n *WarpNode) NodeInfo() warpnet.NodeInfo {
	if n == nil || n.node == nil || n.node.Network() == nil || n.node.Peerstore() == nil {
		return warpnet.NodeInfo{}
	}
	relayState := "Waiting..."

	addrs := n.node.Peerstore().Addrs(n.node.ID())
	addresses := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if !warpnet.IsPublicMultiAddress(a) {
			continue
		}
		if strings.Contains(a.String(), "p2p-circuit") {
			relayState = "Running"
			continue
		}
		addresses = append(addresses, a.String())
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

	if n.relay != nil {
		_ = n.relay.Close()
	}

	if err := n.node.Close(); err != nil {
		log.Errorf("node: failed to close: %v", err)
	}
	n.isClosed.Store(true)
	n.node = nil
	n.addrManager = nil
	return
}
