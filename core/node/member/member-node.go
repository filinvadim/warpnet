package member

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/relay"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/libp2p/go-libp2p/core/peerstore"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync/atomic"
	"time"
)

type PersistentLayer interface {
	warpnet.WarpBatching
	warpnet.WarpProviderStore
	GetOwner() domain.Owner
	SessionToken() string
	PrivateKey() crypto.PrivateKey
	ListProviders() (_ map[string][]warpnet.PeerAddrInfo, err error)
	AddInfo(ctx context.Context, peerId warpnet.WarpPeerID, info p2p.NodeInfo) error
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
	Send(peerAddr warpnet.PeerAddrInfo, r stream.WarpRoute, data []byte) ([]byte, error)
}

type WarpNode struct {
	ctx       context.Context
	node      warpnet.P2PNode
	discovery DiscoveryServicer
	relay     warpnet.WarpRelayCloser
	streamer  Streamer
	isClosed  *atomic.Bool

	ipv4, ipv6 string

	retrier        retrier.Retrier
	ownerId        string
	version        *semver.Version
	selfHash       string
	bootstrapAddrs map[warpnet.WarpPeerID][]warpnet.WarpAddress
}

func NewMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash string,
	db PersistentLayer,
	routingFn routingFunc,
) (_ *WarpNode, err error) {
	store, err := warpnet.NewPeerstore(ctx, db)
	if err != nil {
		return nil, err
	}

	n, err := setupMemberNode(
		ctx, privKey, selfHash, store, config.ConfigFile, routingFn,
	)
	if err != nil {
		return nil, err
	}

	owner := db.GetOwner()
	n.ownerId = owner.UserId

	return n, err
}

type routingFunc func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error)

func setupMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash string,
	store warpnet.WarpPeerstore,
	conf config.Config,
	routingFn routingFunc,
) (*WarpNode, error) {
	node, err := p2p.NewP2PNode(
		privKey,
		store,
		fmt.Sprintf("/ip4/%s/tcp/%s", conf.Node.Host, conf.Node.Port),
		conf,
		routingFn,
	)
	if err != nil {
		return nil, err
	}

	nodeRelay, err := relay.NewRelay(node)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		ctx:            ctx,
		node:           node,
		relay:          nodeRelay,
		isClosed:       new(atomic.Bool),
		retrier:        retrier.New(time.Second * 5),
		version:        conf.Version,
		streamer:       stream.NewStreamPool(ctx, node),
		selfHash:       selfHash,
		bootstrapAddrs: make(map[warpnet.WarpPeerID][]warpnet.WarpAddress),
	}

	n.ipv4, n.ipv6 = parseAddresses(node)

	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		n.bootstrapAddrs[info.ID] = info.Addrs
	}

	println()
	fmt.Printf("\033[1mNODE STARTED WITH ID %s AND ADDRESSES %s %s\033[0m\n", n.ID(), n.ipv4, n.ipv6)
	println()

	return n, nil
}

const localhost = "127.0.0.1"

func (n *WarpNode) Connect(p warpnet.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}

	if len(p.Addrs) == 1 && strings.Contains(p.Addrs[0].String(), localhost) {
		return nil
	}

	if len(p.Addrs) > 1 && strings.Contains(p.Addrs[0].String(), localhost) {
		p.Addrs = p.Addrs[1:]
	}

	bAddrs, ok := n.bootstrapAddrs[p.ID]
	if ok {
		// update local MDNS bootstrap addresses with public ones
		p.Addrs = append(p.Addrs, bAddrs...)
		n.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
	}

	now := time.Now()
	err := n.retrier.Try(
		func() (bool, error) {
			log.Infoln("connect attempt to node:", p.ID.String(), p.Addrs)
			if err := n.node.Connect(n.ctx, p); err != nil {
				return false, nil
			}
			log.Infoln("connect attempt successful:", p.ID.String())
			return true, nil
		},
		now.Add(time.Minute/2),
	)
	return err
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
		if strings.HasPrefix(string(p), "/warpnet") {
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

func (n *WarpNode) NodeInfo() p2p.NodeInfo {
	reg := n.Node()
	id := reg.ID()
	latency := reg.Peerstore().LatencyEWMA(id)
	peerInfo := reg.Peerstore().PeerInfo(id)
	connectedness := reg.Network().Connectedness(id)

	plainAddrs := make([]string, 0, len(peerInfo.Addrs))
	for _, a := range peerInfo.Addrs {
		plainAddrs = append(plainAddrs, a.String())
	}

	return p2p.NodeInfo{
		Addrs:        n.Addrs(),
		Protocols:    n.SupportedProtocols(),
		Latency:      latency,
		NetworkState: connectedness.String(),
		Version:      n.version,
		//StreamStats:  nil, // will be added later
		OwnerId:  n.ownerId,
		SelfHash: n.selfHash,
	}
}

func (n *WarpNode) SelfHash() string {
	return n.selfHash
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

func (n *WarpNode) Mux() warpnet.WarpProtocolSwitch {
	return n.node.Mux()
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

type streamNodeID = string

func (n *WarpNode) GenericStream(nodeIdStr streamNodeID, path stream.WarpRoute, data any) (_ []byte, err error) {
	if n == nil || n.streamer == nil {
		return nil, errors.New("node is not initialized")
	}

	nodeId := warpnet.FromStringToPeerID(nodeIdStr)
	if nodeId == "" {
		return nil, fmt.Errorf("invalid node id: %v", nodeIdStr)
	}
	if n.ID() == nodeId {
		return nil, nil // self request discarded
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

	peerInfo := n.Peerstore().PeerInfo(nodeId)
	if len(peerInfo.Addrs) == 0 {
		log.Debugf("peer %v does not have any addresses: %v", nodeId, peerInfo.Addrs)
		return nil, warpnet.ErrNodeIsOffline
	}
	for _, addr := range peerInfo.Addrs {
		log.Infof("new node is dialable: %s %t\n", addr.String(), n.Network().CanDial(peerInfo.ID, addr))
	}

	return n.streamer.Send(peerInfo, path, bt)
}

func (n *WarpNode) Stop() {
	log.Infoln("shutting down node...")
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered: %v\n", r)
		}
	}()
	if n.relay != nil {
		_ = n.relay.Close()
	}
	if err := n.node.Close(); err != nil {
		log.Errorf("failed to close node: %v", err)
	}
	n.isClosed.Store(true)
	n.node = nil
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
