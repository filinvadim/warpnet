package main

import (
	"context"
	"fmt"
	root "github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node/bootstrap"
	"github.com/filinvadim/warpnet/security"
	log "github.com/sirupsen/logrus"
	_ "go.uber.org/automaxprocs" // DO NOT remove
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Infoln("Warpnet version:", config.ConfigFile.Version)

	psk, err := security.GeneratePSK(root.GetCodeBase(), config.ConfigFile.Version)
	if err != nil {
		panic(err)
	}

	// TODO remove
	fmt.Println("GENERATED PSK:", psk.String())

	log.Infoln("bootstrap nodes: ", config.ConfigFile.Node.Bootstrap)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := bootstrap.NewBootstrapNode(ctx, psk)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}
	defer n.Stop()

	err = n.Start()
	if err != nil {
		log.Fatalf("failed to start bootstrap node: %v", err)
	}

	<-interruptChan
	log.Infoln("bootstrap node interrupted...")
}
