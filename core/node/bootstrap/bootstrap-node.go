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
	log "github.com/sirupsen/logrus"
)

type WarpBootstrapNode struct {
	ctx      context.Context
	node     warpnet.P2PNode
	relay    warpnet.WarpRelayCloser
	version  *semver.Version
	selfHash string
}

type routingFunc func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error)

func NewBootstrapNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash string,
	memoryStore warpnet.WarpPeerstore,
	routingF routingFunc,
) (_ *WarpBootstrapNode, err error) {
	return setupBootstrapNode(
		ctx,
		privKey,
		selfHash,
		memoryStore,
		config.ConfigFile,
		routingF,
	)
}

func setupBootstrapNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	selfHash string,
	store warpnet.WarpPeerstore,
	conf config.Config,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
) (*WarpBootstrapNode, error) {
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

	n := &WarpBootstrapNode{
		ctx:      ctx,
		node:     node,
		relay:    nodeRelay,
		version:  conf.Version,
		selfHash: selfHash,
	}

	println()
	fmt.Printf("\033[1mBOOTSTRAP NODE STARTED WITH ID %s AND ADDRESSES %v\033[0m\n", n.node.ID(), n.node.Addrs())
	println()

	return n, nil
}

func (n *WarpBootstrapNode) SetStreamHandler(route stream.WarpRoute, handler warpnet.WarpStreamHandler) {
	if !stream.IsValidRoute(route) {
		log.Fatalf("invalid route: %v", route)
	}
	n.node.SetStreamHandler(route.ProtocolID(), handler)
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

func (n *WarpBootstrapNode) GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error) {
	panic("just a stub")
}

func (n *WarpBootstrapNode) Stop() {
	if n == nil {
		return
	}
	if err := n.node.Close(); err != nil {
		log.Infoln("bootstrap node stop fail:", err)
	}
	log.Infoln("bootstrap node stopped")
	n.node = nil
}
