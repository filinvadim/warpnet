package bootstrap

import (
	"context"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/relay"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/retrier"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"

	"log"
	"time"
)

type WarpBootstrapNode struct {
	ctx     context.Context
	node    warpnet.P2PNode
	relay   warpnet.WarpRelayCloser
	retrier retrier.Retrier
	version *semver.Version
}

type routingFunc func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error)

func NewBootstrapNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	memoryStore warpnet.WarpPeerstore,
	conf config.Config,
	routingF routingFunc,
) (_ *WarpBootstrapNode, err error) {

	bootstrapAddrs, err := conf.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	return setupBootstrapNode(
		ctx,
		privKey,
		memoryStore,
		bootstrapAddrs,
		conf,
		routingF,
		conf.Version,
	)
}

func setupBootstrapNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	store warpnet.WarpPeerstore,
	addrInfos []warpnet.PeerAddrInfo,
	conf config.Config,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
	version *semver.Version,
) (*WarpBootstrapNode, error) {
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

	basichost.DefaultNegotiationTimeout = 360 * time.Second

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

	n := &WarpBootstrapNode{
		ctx:     ctx,
		node:    node,
		relay:   nodeRelay,
		retrier: retrier.New(time.Second * 5),
		version: version,
	}

	println()
	fmt.Printf("\033[1mBOOTSTRAP NODE STARTED WITH ID %s AND ADDRESSES %v\033[0m\n", n.node.ID(), n.node.Addrs())
	println()

	return n, nil
}

func (n *WarpBootstrapNode) Node() warpnet.P2PNode {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node
}

func (n *WarpBootstrapNode) Addrs() (addrs []string) {
	if n == nil || n.node == nil {
		return nil
	}
	for _, addr := range n.node.Addrs() {
		addrs = append(addrs, addr.String())
	}
	return addrs
}

func (n *WarpBootstrapNode) ID() warpnet.WarpPeerID {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID()
}

func (n *WarpBootstrapNode) Connect(p warpnet.PeerAddrInfo) error {
	if n == nil || n.node == nil {
		return nil
	}

	return n.node.Connect(n.ctx, p)
}

func (n *WarpBootstrapNode) GenericStream(nodeId string, path stream.WarpRoute, data []byte) ([]byte, error) {
	// just a stub
	return nil, nil
}

func (n *WarpBootstrapNode) Stop() {
	if n == nil {
		return
	}
	if err := n.node.Close(); err != nil {
		log.Println("bootstrap node stop fail:", err)
	}
}
