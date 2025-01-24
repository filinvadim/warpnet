package main

import (
	"context"
	"github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
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

	log.Println("Warpnet Version:", warpnet.GetVersion())
	log.Println("config bootstrap nodes: ", conf.Node.Bootstrap)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("starting bootstrap node...")

	n, err := node.NewBootstrapNode(ctx, conf)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}

	defer n.Stop()

	<-interruptChan
	log.Println("bootstrap node interrupted...")
}
