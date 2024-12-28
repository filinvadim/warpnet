package main

import (
	"context"
	"flag"
	"github.com/filinvadim/warpnet/core/node"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "go.uber.org/automaxprocs"
)

var (
	version = "0.0.1"
)

func main() {
	id := new(string)
	id = flag.String("id", "", "node ID")
	flag.Parse()

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("starting bootstrap node...")

	n, err := node.NewBootstrapNode(ctx, *id)
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
