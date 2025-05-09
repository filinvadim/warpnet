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
	log "github.com/sirupsen/logrus"
	"strings"
	"sync/atomic"
	"time"
)

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
	relay       warpnet.WarpRelayCloser
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
	limiter := warpnet.NewFixedLimiter()

	manager, err := warpnet.NewConnManager(limiter)
	if err != nil {
		return nil, err
	}

	rm, err := warpnet.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	addrManager := NewAddressManager()

	infos, err := config.ConfigFile.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	currentNodeID, err := warpnet.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	reachibilityOption := libp2p.ForceReachabilityPrivate
	autotaticRelaysOption := EnableAutoRelayWithStaticRelays(infos, currentNodeID)
	natServiceOption := DisableOption()
	natPortMapOption := libp2p.NATPortMap
	if ownerId == warpnet.BootstrapOwner {
		reachibilityOption = libp2p.ForceReachabilityPublic
		autotaticRelaysOption = DisableOption()
		natServiceOption = libp2p.EnableNATService
		natPortMapOption = DisableOption()
	}

	node, err := warpnet.NewP2PNode(
		libp2p.WithDialTimeout(DefaultTimeout),
		libp2p.ListenAddrStrings(
			listenAddr,
		),
		libp2p.AddrsFactory(addrManager.Factory()),
		libp2p.SwarmOpts(
			WithDialTimeout(DefaultTimeout),
			WithDialTimeoutLocal(DefaultTimeout),
		),
		libp2p.Transport(warpnet.NewTCPTransport, WithDefaultTCPConnectionTimeout(DefaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(warpnet.NoiseID, warpnet.NewNoise),
		libp2p.Peerstore(store),
		libp2p.ResourceManager(rm),
		libp2p.PrivateNetwork(warpnet.PSK(psk)),
		libp2p.UserAgent(ServiceName),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),

		libp2p.EnableAutoNATv2(),
		libp2p.EnableRelay(),
		libp2p.EnableRelayService(relay.WithDefaultResources()), // for member nodes that have static IP
		libp2p.EnableHolePunching(),

		natServiceOption(),
		natPortMapOption(),
		autotaticRelaysOption(),
		reachibilityOption(),
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

func (n *WarpNode) Connect(p warpnet.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}

	peerState := n.node.Network().Connectedness(p.ID)
	isConnected := peerState == warpnet.Connected || peerState == warpnet.Limited
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

const (
	relayStatusWaiting = "waiting"
	relayStatusRunning = "running"
)

func (n *WarpNode) NodeInfo() warpnet.NodeInfo {
	if n == nil || n.node == nil || n.node.Network() == nil || n.node.Peerstore() == nil {
		return warpnet.NodeInfo{}
	}
	relayState := relayStatusWaiting

	addrs := n.node.Peerstore().Addrs(n.node.ID())
	addresses := make([]string, 0, len(addrs))
	for _, ma := range addrs {
		if !warpnet.IsPublicMultiAddress(ma) {
			continue
		}
		if warpnet.IsRelayMultiaddress(ma) {
			relayState = relayStatusRunning
		}
		addresses = append(addresses, ma.String())
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

var ErrSelfRequest = errors.New("self request is not allowed")

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

const default4001Port = "4001"

func (n *WarpNode) AddOwnPublicAddress(remoteAddr string) error {
	if remoteAddr == "" {
		return errors.New("node: empty node info public address")
	}
	maddr, err := warpnet.NewMultiaddr(remoteAddr)
	if err != nil {
		return err
	}

	proto := "ip4"
	ipStr, err := maddr.ValueForProtocol(warpnet.P_IP4)
	if err != nil {
		proto = "ip6"
		ipStr, err = maddr.ValueForProtocol(warpnet.P_IP6)
		if err != nil {
			return err
		}
	}

	newMAddr, err := warpnet.NewMultiaddr(
		fmt.Sprintf("/%s/%s/tcp/%s", proto, ipStr, default4001Port),
	)
	if err != nil {
		return err
	}

	week := time.Hour * 24 * 7
	n.Peerstore().AddAddr(n.node.ID(), maddr, week)
	n.Peerstore().AddAddr(n.node.ID(), newMAddr, week)
	n.addrManager.Add(maddr, week)
	n.addrManager.Add(newMAddr, week)
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
