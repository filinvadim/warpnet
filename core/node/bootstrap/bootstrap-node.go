package bootstrap

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/handler"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/node/base"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/security"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

type BootstrapNode struct {
	*base.WarpNode

	discService       DiscoveryHandler
	mdnsService       MDNSStarterCloser
	pubsubService     PubSubProvider
	raft              ConsensusProvider
	dHashTable        DistributedHashTableCloser
	providerStore     ProviderCacheCloser
	memoryStoreCloseF func() error
	psk               security.PSK
}

func NewBootstrapNode(
	ctx context.Context,
	psk security.PSK,
) (_ *BootstrapNode, err error) {
	seed := []byte("bootstrap")
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		seed = []byte(hostname)
	}
	privKey, err := security.GenerateKeyFromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: fail generating key: %v", err)
	}
	warpPrivKey := privKey.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: fail getting ID: %v", err)
	}

	discService := discovery.NewBootstrapDiscoveryService(ctx)
	raft, err := consensus.NewBootstrapRaft(ctx)
	if err != nil {
		return nil, err
	}

	mdnsService := mdns.NewMulticastDNS(ctx, discService.DefaultDiscoveryHandler, raft.AddVoter)
	pubsubService := pubsub.NewPubSubBootstrap(ctx, discService.DefaultDiscoveryHandler, raft.AddVoter)

	memoryStore, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, fmt.Errorf("bootstrap: fail creating memory peerstore: %w", err)
	}

	mapStore := datastore.NewMapDatastore()

	closeF := func() error {
		memoryStore.Close()
		return mapStore.Close()
	}

	providersCache, err := providers.NewProviderManager(id, memoryStore, mapStore)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: fail creating providers cache: %w", err)
	}

	dHashTable := dht.NewDHTable(
		ctx, mapStore, providersCache,
		raft.RemoveVoter, discService.DefaultDiscoveryHandler, raft.AddVoter,
	)

	node, err := base.NewWarpNode(
		ctx,
		warpPrivKey,
		memoryStore,
		warpnet.BootstrapOwner,
		psk,
		fmt.Sprintf("/ip4/%s/tcp/%s", config.ConfigFile.Node.Host, config.ConfigFile.Node.Port),
		dHashTable.StartRouting,
	)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: failed to init node: %v", err)
	}

	println()
	fmt.Printf(
		"\033[1mBOOTSTRAP NODE STARTED WITH ID %s AND ADDRESSES %v\033[0m\n",
		node.NodeInfo().ID, node.NodeInfo().Addresses,
	)
	println()

	bn := &BootstrapNode{
		WarpNode:          node,
		discService:       discService,
		mdnsService:       mdnsService,
		pubsubService:     pubsubService,
		raft:              raft,
		dHashTable:        dHashTable,
		providerStore:     providersCache,
		memoryStoreCloseF: closeF,
		psk:               psk,
	}

	mw := middleware.NewWarpMiddleware()
	logMw := mw.LoggingMiddleware
	bn.SetStreamHandler(
		event.PUBLIC_POST_NODE_VERIFY,
		mw.LoggingMiddleware(mw.UnwrapStreamMiddleware(handler.StreamVerifyHandler(bn.raft))),
	)
	bn.SetStreamHandler(
		event.PUBLIC_GET_INFO,
		logMw(handler.StreamGetInfoHandler(bn, nil)),
	)

	return bn, nil
}

func (bn *BootstrapNode) Start() error {
	go bn.discService.Run(bn)
	go bn.mdnsService.Start(bn)
	go bn.pubsubService.Run(bn, nil)

	if err := bn.raft.Sync(bn); err != nil {
		return err
	}

	log.Debugln("SUPPORTED PROTOCOLS:", strings.Join(bn.SupportedProtocols(), ","))
	return nil
}

func (bn *BootstrapNode) GenericStream(nodeIdStr string, path stream.WarpRoute, data any) (_ []byte, err error) {
	// stub
	return nil, nil
}

func (bn *BootstrapNode) Stop() {
	if bn == nil {
		return
	}
	if bn.discService != nil {
		bn.discService.Close()
	}
	if bn.mdnsService != nil {
		bn.mdnsService.Close()
	}
	if bn.pubsubService != nil {
		if err := bn.pubsubService.Close(); err != nil {
			log.Errorf("bootstrap: failed to close pubsub: %v", err)
		}
	}
	if bn.providerStore != nil {
		if err := bn.providerStore.Close(); err != nil {
			log.Errorf("bootstrap: failed to close provider: %v", err)
		}
	}
	if bn.dHashTable != nil {
		bn.dHashTable.Close()
	}
	if bn.raft != nil {
		bn.raft.Shutdown()
	}
	if bn.memoryStoreCloseF != nil {
		if err := bn.memoryStoreCloseF(); err != nil {
			log.Errorf("bootstrap: failed to close memory store: %v", err)
		}
	}

	bn.WarpNode.StopNode()
}
