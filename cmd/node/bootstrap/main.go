package main

import (
	"context"
	root "github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/node/bootstrap"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/security"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	log "github.com/sirupsen/logrus"
	_ "go.uber.org/automaxprocs" // DO NOT remove
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Infoln("Warpnet version:", config.ConfigFile.Version)

	codeHash, err := security.GetCodebaseHash(root.GetCodeBase())
	if err != nil {
		panic(err)
	}

	log.Infof("codebase hash: %x", codeHash)
	log.Infoln("bootstrap nodes: ", config.ConfigFile.Node.Bootstrap)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	privKey, err := security.GenerateKeyFromSeed([]byte(config.ConfigFile.Node.Host))
	if err != nil {
		log.Fatalf("fail generating key: %v", err)
	}
	warpPrivKey := privKey.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
	if err != nil {
		log.Fatalf("fail getting ID: %v", err)
	}

	mdnsService := mdns.NewMulticastDNS(ctx, nil)
	defer mdnsService.Close()
	pubsubService := pubsub.NewPubSub(ctx, nil)

	memoryStore, err := pstoremem.NewPeerstore()
	if err != nil {
		log.Fatalf("fail creating memory peerstore: %v", err)
	}
	defer memoryStore.Close()

	mapStore := datastore.NewMapDatastore()
	defer mapStore.Close()

	providersCache, err := providers.NewProviderManager(id, memoryStore, mapStore)
	if err != nil {
		log.Fatalf("fail creating providers cache: %v", err)
	}
	defer providersCache.Close()

	raft, err := consensus.NewRaft(ctx, nil, true)
	if err != nil {
		log.Fatalln(err)
	}

	dHashTable := dht.NewDHTable(
		ctx, mapStore, providersCache, codeHash,
		raft.AddVoter, raft.RemoveVoter,
	)
	defer dHashTable.Close()

	n, err := bootstrap.NewBootstrapNode(
		ctx, warpPrivKey, string(codeHash), memoryStore, dHashTable.StartRouting,
	)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}
	defer n.Stop()

	go mdnsService.Start(n)
	go pubsubService.Run(n, nil, nil, nil)
	defer pubsubService.Close()

	raft.Negotiate(n)
	defer raft.Shutdown()

	<-interruptChan
	log.Infoln("bootstrap node interrupted...")
}
