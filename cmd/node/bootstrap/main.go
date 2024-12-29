package main

import (
	"context"
	_ "embed"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "go.uber.org/automaxprocs"
)

var (
	version = "0.0.1"
)

//go:embed config.yml
var configFile []byte

func main() {
	var conf config.Config
	if err := yaml.Unmarshal(configFile, &conf); err != nil {
		log.Fatal("unmarshalling config: ", err)
	}

	version = conf.Version.String()
	log.Println("config bootstrap nodes: ", conf.Node.BootstrapAddrs)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	})
	go http.ListenAndServe(":8080", nil)

	log.Println("starting bootstrap node...")

	n, err := node.NewBootstrapNode(ctx, conf)
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
