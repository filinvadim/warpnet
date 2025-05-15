/*

 Warpnet - Decentralized Social Network
 Copyright (C) 2025 Vadim Filin, https://github.com/filinvadim,
 <github.com.mecdy@passmail.net>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"crypto/rand"
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

	seed := []byte(rand.Text())
	n, err := bootstrap.NewBootstrapNode(ctx, true, seed, psk)
	if err != nil {
		log.Fatalf("failed to init bootstrap node: %v", err)
	}
	defer n.Stop()

	if err := n.Start(); err != nil {
		log.Fatalf("failed to start bootstrap node: %v", err)
	}

	m := metrics.NewMetricsClient(config.ConfigFile.Node.Metrics.Server, n.NodeInfo().ID.String(), true)
	m.PushStatusOnline()

	// TODO business node is here

	<-interruptChan
	log.Infoln("business node interrupted...")
}

// TODO temp. Check for https://github.com/libp2p/go-libp2p-kad-dht/issues/1073
func closeWriter() {
	defer func() { recover() }()
	_ = writer.WriterGroup.Close()
}
