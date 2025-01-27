package bootstrap

import (
	"context"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/dht-table"
	"github.com/filinvadim/warpnet/core/encrypting"
	node2 "github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/ipfs/go-datastore"
	log2 "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
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

func NewBootstrapNode(
	ctx context.Context,
	conf config.Config,
) (_ *WarpBootstrapNode, err error) {
	privKey, err := encrypting.GenerateKeyFromSeed([]byte("bootstrap")) // TODO
	if err != nil {
		return nil, err
	}

	warpPrivKey := privKey.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
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

	hTable := dht_table.NewDHTable(
		ctx, mapStore, providersCache, bootstrapAddrs,
		func(info warpnet.PeerAddrInfo) {
			log.Println("dht: node added", info.ID)
		},
		func(info warpnet.PeerAddrInfo) {
			log.Println("dht: node removed", info.ID)
		},
	)

	return setupBootstrapNode(
		ctx,
		warpPrivKey,
		store,
		bootstrapAddrs,
		conf,
		hTable.Start,
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
	log2.SetLogLevel("*", "INFO")

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

	node, err := node2.NewP2PNode(
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
	relay, err := relayv2.New(
		node,
		relayv2.WithInfiniteLimits(),
	)
	if err != nil {
		return nil, err
	}

	n := &WarpBootstrapNode{
		ctx:     ctx,
		node:    node,
		relay:   relay,
		retrier: retrier.New(time.Second * 5),
		version: version,
	}

	println()
	fmt.Printf("\033[1mBOOTSTRAP NODE STARTED WITH ID %s AND ADDRESSES %s %v\033[0m\n", n.node.ID(), n.node.Addrs())
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

func (n *WarpBootstrapNode) Stop() {
	if n == nil {
		return
	}
	if err := n.node.Close(); err != nil {
		log.Println("bootstrap node stop fail:", err)
	}
}
