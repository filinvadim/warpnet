package dhash_table

import (
	"context"
	"errors"
	"fmt"
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

var ErrDHTMisconfigured = errors.New("DHT is misconfigured")

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
		log.Infof("dht: new: %v", err)
		return nil, err
	}

	d.dht.RoutingTable().PeerAdded = defaultNodeAddedCallback
	if d.addFuncs != nil {
		d.dht.RoutingTable().PeerAdded = func(id peer.ID) {
			log.Infof("dht: new peer added: %s", id)
			info := peer.AddrInfo{ID: id}
			for _, addF := range d.addFuncs {
				addF(info)
			}
			<-d.dht.RefreshRoutingTable()
		}
	}
	d.dht.RoutingTable().PeerRemoved = defaultNodeRemovedCallback
	if d.removeF != nil {
		d.dht.RoutingTable().PeerRemoved = func(id peer.ID) {
			d.removeF(id)
		}
		<-d.dht.RefreshRoutingTable()
	}

	go d.setupDHT()

	<-d.dht.RefreshRoutingTable()

	log.Infoln("dht: routing started")

	return d.dht, nil
}

func (d *DistributedHashTable) setupDHT() {
	if d == nil || d.dht != nil {
		return
	}
	// force node to know its external address (in case of local network)
	for _, info := range d.boostrapNodes {
		d.dht.Host().Peerstore().AddAddrs(info.ID, info.Addrs, warpnet.PermanentAddrTTL)
	}

	if err := d.dht.Bootstrap(d.ctx); err != nil {
		log.Errorf("dht: bootstrap: %s", err)
	}

	d.correctPeerIdMismatch(d.boostrapNodes)

	<-d.dht.RefreshRoutingTable()

	go d.runRendezvousDiscovery()
}

func (d *DistributedHashTable) runRendezvousDiscovery() {
	routingDiscovery := drouting.NewRoutingDiscovery(d.dht)
	_, err := routingDiscovery.Advertise(d.ctx, WarpnetRendezvous, lip2pDiscovery.TTL(time.Hour))
	if err != nil {
		log.Errorf("dht rendezvous: advertise: %s", err)
		return
	}

	peerChan, err := routingDiscovery.FindPeers(d.ctx, WarpnetRendezvous, lip2pDiscovery.TTL(time.Hour))
	if err != nil {
		log.Errorf("dht rendezvous: find peers: %s", err)
		return
	}

	log.Infoln("dht rendezvous: is running")

	select {
	case peerInfo := <-peerChan:
		log.Infof("dht rendezvous: found new peer: %s", peerInfo.String())

		for _, addF := range d.addFuncs {
			addF(peerInfo)
		}
	case <-d.stopChan:
		return
	case <-d.ctx.Done():
		return
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

	fmt.Println("??????????????")
	if err := d.dht.Close(); err != nil {
		log.Errorf("dht: table close: %v\n", err)
	}
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@")
	d.dht = nil
	close(d.stopChan)
	log.Infoln("dht: table closed")
}
