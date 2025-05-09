package main

import (
	"context"
	root "github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node/bootstrap"
	"github.com/filinvadim/warpnet/metrics"
	"github.com/filinvadim/warpnet/security"
	writer "github.com/ipfs/go-log/writer"
	log "github.com/sirupsen/logrus"
	_ "go.uber.org/automaxprocs" // DO NOT remove
	"os"
	"os/signal"
	"syscall"
)

func main() {
	defer closeWriter()

	log.Infoln("Warpnet version:", config.ConfigFile.Version)

	psk, err := security.GeneratePSK(root.GetCodeBase(), config.ConfigFile.Version)
	if err != nil {
		panic(err)
	}

	log.Infoln("bootstrap nodes: ", config.ConfigFile.Node.Bootstrap)

	lvl, err := log.ParseLevel(config.ConfigFile.Logging.Level)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)

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

	m := metrics.NewMetricsClient(config.ConfigFile.Node.Metrics.Server, n.NodeInfo().ID.String(), true)
	m.PushStatusOnline()
	<-interruptChan
	log.Infoln("bootstrap node interrupted...")
}

// TODO temp. Check for https://github.com/libp2p/go-libp2p-kad-dht/issues/1073
func closeWriter() {
	defer func() { recover() }()
	_ = writer.WriterGroup.Close()
}
