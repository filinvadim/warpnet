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

// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package dht

import (
	"context"
	"errors"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/discovery"
	lip2pDiscovery "github.com/libp2p/go-libp2p/core/discovery"

	"github.com/filinvadim/warpnet/core/warpnet"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/sec"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"time"
)

/*
  Distributed Hash Table (DHT) is a distributed hash table used for decentralized
  data storage and lookup in peer-to-peer (P2P) networks. Instead of storing data on a single server,
  DHT distributes it across multiple nodes.

  DHT solves three main tasks:
  1. Routing — enables efficient lookup of nodes storing specific keys.
  2. Data storage — each node is responsible for a portion of the key space.
  3. Key-based lookup — provides fast access to data without a central server.

  DHT is used in BitTorrent, IPFS, Ethereum, as well as in P2P messengers and other decentralized applications.

  The go-libp2p-kad-dht library is an implementation of Kademlia DHT for libp2p.
  It allows peer-to-peer nodes to exchange data and discover each other without centralized servers.

  Key features of go-libp2p-kad-dht:
  - **Kademlia Algorithm**
    - Implements Kademlia DHT, one of the most widely used algorithms for distributed hash tables.
  - **Node and data lookup in a P2P network**
    - Enables finding nodes and querying them for data by key.
  - **Flexible routing**
    - Optimized for dynamic networks where nodes frequently join and leave.
  - **Support for PubSub and IPFS**
    - Used in IPFS and applicable to P2P messengers and decentralized applications.
  - **Key hashing**
    - Distributes the key space across nodes, ensuring balanced load distribution.

  DHT is well-suited for decentralized applications that require distributed search without a single point of failure,
  P2P networks where nodes frequently connect and disconnect, and data exchange between nodes without a central server.

  The go-libp2p-kad-dht library is useful for finding other nodes in a libp2p network,
  implementing decentralized content lookup (as in IPFS), and enabling efficient routing in a distributed network.
*/

const WarpnetRendezvous = "rendezvous-point@warpnet"

type RoutingStorer interface {
	warpnet.WarpBatching
}

type ProviderStorer interface {
	AddProvider(ctx context.Context, key []byte, prov peer.AddrInfo) error
	GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error)
	io.Closer
}

type DistributedHashTable struct {
	ctx           context.Context
	db            RoutingStorer
	providerStore ProviderStorer
	boostrapNodes []warpnet.PeerAddrInfo
	addFuncs      []discovery.DiscoveryHandler
	removeF       func(warpnet.WarpPeerID)
	dht           *dht.IpfsDHT
	stopChan      chan struct{}
	cancelFunc    context.CancelFunc
}

func defaultNodeRemovedCallback(id warpnet.WarpPeerID) {
	log.Debugln("dht: node removed", id)
}

func defaultNodeAddedCallback(id warpnet.WarpPeerID) {
	log.Debugln("dht: node added", id)
}

func NewDHTable(
	ctx context.Context,
	db RoutingStorer,
	providerStore ProviderStorer,
	removeF func(warpnet.WarpPeerID),
	addFuncs ...discovery.DiscoveryHandler,
) *DistributedHashTable {
	bootstrapAddrs, _ := config.ConfigFile.Node.AddrInfos()
	log.Infoln("dht: bootstrap addresses:", bootstrapAddrs)
	return &DistributedHashTable{
		ctx:           ctx,
		db:            db,
		providerStore: providerStore,
		boostrapNodes: bootstrapAddrs,
		addFuncs:      addFuncs,
		removeF:       removeF,
		stopChan:      make(chan struct{}),
	}
}

func (d *DistributedHashTable) StartRouting(n warpnet.P2PNode) (_ warpnet.WarpPeerRouting, err error) {
	d.dht, err = dht.New(
		d.ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(protocol.ID("/"+config.ConfigFile.Node.Prefix)),
		dht.Datastore(d.db),
		dht.MaxRecordAge(time.Hour*24*365),
		dht.RoutingTableRefreshPeriod(time.Hour),
		dht.RoutingTableRefreshQueryTimeout(time.Minute*5),
		dht.BootstrapPeers(d.boostrapNodes...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Hour*24),
		dht.BucketSize(50),
	)
	if err != nil {
		log.Errorf("dht: new: %v", err)
		return nil, err
	}

	d.dht.RoutingTable().PeerAdded = defaultNodeAddedCallback
	if d.addFuncs != nil {
		d.dht.RoutingTable().PeerAdded = func(id peer.ID) {
			log.Infof("dht: peer added: %s", id)
			info := peer.AddrInfo{ID: id}
			for _, addF := range d.addFuncs {
				addF(info)
			}
		}
	}
	d.dht.RoutingTable().PeerRemoved = defaultNodeRemovedCallback
	if d.removeF != nil {
		d.dht.RoutingTable().PeerRemoved = func(id peer.ID) {
			log.Infof("dht: peer removed: %s", id)
			d.removeF(id)
		}
	}

	go d.bootstrapDHT()
	log.Infoln("dht: routing started")
	return d.dht, nil
}

func (d *DistributedHashTable) bootstrapDHT() {
	if d == nil || d.dht == nil {
		return
	}
	ownID := d.dht.Host().ID()

	// force dht to know its bootstrap nodes, force libp2p node to know its external address (in case of local network)
	for _, info := range d.boostrapNodes {
		if ownID == info.ID {
			continue
		}
		d.dht.Host().Peerstore().AddAddrs(info.ID, info.Addrs, warpnet.PermanentAddrTTL)
	}

	if err := d.dht.Bootstrap(d.ctx); err != nil {
		log.Errorf("dht: bootstrap: %s", err)
	}

	d.correctPeerIdMismatch(d.boostrapNodes)

	log.Infoln("dht: bootstrap complete")
	<-d.dht.RefreshRoutingTable()

	go d.runRendezvousDiscovery(ownID)
}

// rendezvous discovery is memory leaking so run it only for 5 minutes
func (d *DistributedHashTable) runRendezvousDiscovery(ownID warpnet.WarpPeerID) {
	defer func() { recover() }()
	if d == nil || d.dht == nil {
		return
	}

	defer log.Infoln("dht rendezvous: stopped")

	tryouts := 30
	for len(d.dht.RoutingTable().ListPeers()) == 0 {
		if tryouts == 0 {
			log.Infoln("dht rendezvous: timeout - no peers found")
			return
		}
		time.Sleep(time.Second * 5)
		tryouts--
	}

	timer := time.NewTimer(time.Minute * 5)
	defer timer.Stop()

	rendezvousCtx, cancel := context.WithCancel(context.Background())
	d.cancelFunc = cancel

	routingDiscovery := drouting.NewRoutingDiscovery(d.dht)
	_, err := routingDiscovery.Advertise(
		rendezvousCtx, WarpnetRendezvous, lip2pDiscovery.TTL(time.Hour*3), lip2pDiscovery.Limit(5))
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		log.Errorf("dht rendezvous: advertise: %s", err)
		return
	}

	peerChan, err := routingDiscovery.FindPeers(rendezvousCtx, WarpnetRendezvous)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		log.Errorf("dht rendezvous: find peers: %s", err)
		return
	}
	if peerChan == nil {
		return
	}

	log.Infoln("dht rendezvous: is running")

	for {
		select {
		case <-timer.C:
			return
		case <-d.stopChan:
			return
		case <-rendezvousCtx.Done():
			return
		case peerInfo := <-peerChan:
			if peerInfo.ID == ownID {
				continue
			}
			if len(peerInfo.Addrs) == 0 {
				continue
			}
			log.Infof("dht rendezvous: found new peer: %s", peerInfo.String())
			for _, addF := range d.addFuncs {
				addF(peerInfo)
			}
		}
	}
}

func (d *DistributedHashTable) correctPeerIdMismatch(boostrapNodes []warpnet.PeerAddrInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // common timeout
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	for _, addr := range boostrapNodes {
		addr := addr // this is important!
		g.Go(func() error {
			localCtx, localCancel := context.WithTimeout(ctx, time.Second) // local timeout
			defer localCancel()

			err := d.dht.Ping(localCtx, addr.ID)
			if err == nil {
				return nil
			}
			var pidErr sec.ErrPeerIDMismatch
			if !errors.As(err, &pidErr) {
				return nil
			}

			d.dht.RoutingTable().RemovePeer(pidErr.Expected)
			d.dht.Host().Peerstore().ClearAddrs(pidErr.Expected)
			d.dht.Host().Peerstore().AddAddrs(pidErr.Actual, addr.Addrs, time.Hour*24)
			log.Infof("dht: peer id corrected from %s to %s", pidErr.Expected, pidErr.Actual)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Errorf("dht: mismatch: waitgroup: %v", err)
	}
}

func (d *DistributedHashTable) Close() {
	defer func() { recover() }()
	if d == nil || d.dht == nil {
		return
	}
	if d.cancelFunc != nil {
		d.cancelFunc()
	}
	close(d.stopChan)

	log.Infoln("dht rendezvous: closing...")
	if err := d.dht.Close(); err != nil {
		log.Errorf("dht: table close: %v\n", err)
	}
	log.Infoln("dht: table closed")
}
