package member

import (
	"context"
	go_crypto "crypto"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/relay"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/filinvadim/warpnet/security"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	log "github.com/sirupsen/logrus"
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
	Send(peerAddr *warpnet.PeerAddrInfo, r stream.WarpRoute, data []byte) ([]byte, error)
}

type WarpNode struct {
	ctx       context.Context
	node      warpnet.P2PNode
	discovery DiscoveryServicer
	relay     warpnet.WarpRelayCloser
	streamer  Streamer
	isClosed  *atomic.Bool

	ipv4, ipv6 string

	retrier  retrier.Retrier
	ownerId  string
	version  *semver.Version
	selfHash security.SelfHash
}

func NewMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash security.SelfHash,
	db PersistentLayer,
	conf config.Config,
	routingFn routingFunc,
) (_ *WarpNode, err error) {
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	if err != nil {
		return nil, err
	}

	bootstrapAddrs, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	n, err := setupMemberNode(
		ctx, privKey, selfHash, store, bootstrapAddrs, conf, routingFn,
	)
	if err != nil {
		return nil, err
	}

	owner, _ := db.GetOwner()
	n.ownerId = owner.UserId

	return n, err
}

type routingFunc func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error)

func setupMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash security.SelfHash,
	store warpnet.WarpPeerstore,
	addrInfos []warpnet.PeerAddrInfo,
	conf config.Config,
	routingFn routingFunc,
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

	node, err := p2p.NewP2PNode(
		privKey,
		store,
		addrInfos,
		conf,
		routingFn,
		rm,
		manager,
	)
	if err != nil {
		return nil, err
	}

	nodeRelay, err := relay.NewRelay(node)
	if err != nil {
		return nil, err
	}

	n := &WarpNode{
		ctx:      ctx,
		node:     node,
		relay:    nodeRelay,
		isClosed: new(atomic.Bool),
		retrier:  retrier.New(time.Second * 5),
		version:  conf.Version,
		streamer: stream.NewStreamPool(ctx, node),
		selfHash: selfHash,
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
			log.Infoln("connect attempt to node:", p.ID.String())
			if err := n.node.Connect(n.ctx, p); err != nil {
				log.Errorf("node connect attempt error: %v", err)
				return false, nil
			}
			log.Infoln("connect attempt successful:", p.ID.String())
			return true, nil
		},
		now.Add(time.Minute*2),
	)
	return err
}

func (n *WarpNode) SetStreamHandler(route stream.WarpRoute, handler warpnet.WarpStreamHandler) {
	n.node.SetStreamHandler(route.ProtocolID(), handler)
}

func (n *WarpNode) NodeInfo() p2p.NodeInfo {
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

	return p2p.NodeInfo{
		Addrs:     addrs,
		Protocols: stream.FromPrIDToRoutes(protocols),
		Latency:   latency,
		PeerInfo: warpnet.WarpAddrInfo{
			ID:    peerInfo.ID,
			Addrs: plainAddrs,
		},
		NetworkState: connectedness.String(),
		ListenAddrs:  listenAddrs,
		Version:      n.version.String(),
		//StreamStats:  nil, // will be added later
		OwnerId:  n.ownerId,
		SelfHash: n.selfHash,
	}
}

func (n *WarpNode) SelfHash() security.SelfHash {
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

func (n *WarpNode) Addrs() []string {
	return []string{n.ipv4, n.ipv6}
}
func (n *WarpNode) IPv4() string {
	return n.ipv4
}

func (n *WarpNode) IPv6() string {
	return n.ipv6
}

func (n *WarpNode) GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error) {
	if n == nil || n.streamer == nil {
		return nil, nil
	}

	var bt []byte
	if data != nil {
		var ok bool
		bt, ok = data.([]byte)
		if !ok {
			bt, err = json.JSON.Marshal(data)
			if err != nil {
				return nil, err
			}
		}
	}

	id, err := warpnet.IDFromBytes([]byte(nodeId))
	if err != nil {
		return nil, err
	}
	log.Infoln("stream: request:", path, nodeId)
	return n.streamer.Send(&warpnet.PeerAddrInfo{ID: id}, path, bt)
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
