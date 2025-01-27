package main

import (
	"context"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/node/bootstrap"
	"github.com/filinvadim/warpnet/core/pubsub"
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

	log.Println("starting bootstrap node...")

	mdnsService := mdns.NewMulticastDNS(ctx, nil)
	defer mdnsService.Close()
	pubsubService := pubsub.NewPubSub(ctx, nil)
	defer pubsubService.Close()

	n, err := bootstrap.NewBootstrapNode(ctx, conf)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}
	defer n.Stop()

	go mdnsService.Start(n)
	go pubsubService.RunDiscovery(n)

	<-interruptChan
	log.Println("bootstrap node interrupted...")
}
