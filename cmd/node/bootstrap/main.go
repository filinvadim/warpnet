package main

import (
	"context"
	"fmt"
	embedded "github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/node/bootstrap"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "go.uber.org/automaxprocs" // DO NOT remove
)

func main() {
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatalf("fail loading config: %v", err)
	}

	log.Println("Warpnet Version:", conf.Version)
	log.Println("config bootstrap nodes: ", conf.Node.Bootstrap)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	isValid := encrypting.VerifySelfSignature(embedded.GetSignature(), embedded.GetPublicKey())
	if !isValid {
		log.Println("invalid binary signature - TODO") // TODO
	}

	selfHash, err := encrypting.GetSelfHash(encrypting.Bootstrap)
	if err != nil {
		log.Fatalf("fail to get self hash: %v", err)
	}
	fmt.Println("self hash:", selfHash) // TODO verify with network consensus

	seed := []byte("bootstrap")
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		seed = []byte(hostname)
	}
	privKey, err := encrypting.GenerateKeyFromSeed(seed)
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
	defer pubsubService.Close()

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

	dHashTable := dht.NewDHTable(
		ctx, mapStore, providersCache, conf,
		dht.DefaultNodeAddedCallback,
		dht.DefaultNodeRemovedCallback,
	)
	defer dHashTable.Close()

	n, err := bootstrap.NewBootstrapNode(
		ctx, warpPrivKey, selfHash, memoryStore, conf, dHashTable.StartRouting,
	)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}
	defer n.Stop()

	go mdnsService.Start(n)
	go pubsubService.Run(n)

	raft, err := consensus.NewRaft(ctx, n, nil, nil, true)
	if err != nil {
		log.Fatal(err)
	}
	raft.Start()
	defer raft.Shutdown()
	<-interruptChan
	log.Println("bootstrap node interrupted...")
}
