package main

import (
	"context"
	"github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/logger"
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

	l := logger.NewUnifiedLogger(conf.Node.Logging.Level, true)
	log.Println("starting bootstrap node...")

	n, err := node.NewBootstrapNode(ctx, conf, l)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}

	defer func() {
		if err := n.Stop(); err != nil {
			log.Fatalf("failed to stop bootstrap node: %v", err)
		}
	}()

	<-interruptChan
	log.Println("bootstrap node interrupted...")
}
