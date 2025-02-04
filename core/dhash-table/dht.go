package dhash_table

import (
	"context"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	log "github.com/sirupsen/logrus"
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
	db            datastore.Batching
	providerStore providers.ProviderStore
	boostrapNodes []warpnet.PeerAddrInfo
	addF          discovery.DiscoveryHandler
	removeF       discovery.DiscoveryHandler
	dht           *dht.IpfsDHT
	prefix        string
}

func DefaultNodeRemovedCallback(info warpnet.PeerAddrInfo) {
	log.Infoln("dht: node removed", info.ID)
}

// TODO: track this: dht	go-libp2p-kad-dht/dht.go:523
// failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
// "error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
// all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
// peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
// but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}
func DefaultNodeAddedCallback(info warpnet.PeerAddrInfo) {
	log.Infoln("dht: node added", info.ID)
}

func NewDHTable(
	ctx context.Context,
	db RoutingStorer,
	providerStore ProviderStorer,
	addF discovery.DiscoveryHandler,
	removeF discovery.DiscoveryHandler,
) *DistributedHashTable {
	bootstrapAddrs, _ := config.ConfigFile.Node.AddrInfos()
	return &DistributedHashTable{
		ctx:           ctx,
		db:            db,
		providerStore: providerStore,
		boostrapNodes: bootstrapAddrs,
		addF:          addF,
		removeF:       removeF,
		prefix:        "/" + config.ConfigFile.Node.Prefix,
	}
}

func (d *DistributedHashTable) StartRouting(n warpnet.P2PNode) (_ warpnet.WarpPeerRouting, err error) {
	dhTable, err := dht.New(
		d.ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(protocol.ID(d.prefix)),
		dht.Datastore(d.db),
		dht.MaxRecordAge(time.Hour*24*365),
		dht.RoutingTableRefreshPeriod(time.Hour*24),
		dht.RoutingTableRefreshQueryTimeout(time.Hour*24),
		dht.BootstrapPeers(d.boostrapNodes...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Hour*24),
	)
	if err != nil {
		log.Infof("new dht: %v\n", err)
		return nil, err
	}

	if d.addF != nil {
		dhTable.RoutingTable().PeerAdded = func(id peer.ID) {
			d.addF(peer.AddrInfo{ID: id})
		}
	}
	if d.removeF != nil {
		dhTable.RoutingTable().PeerRemoved = func(id peer.ID) {
			d.removeF(peer.AddrInfo{ID: id})
		}
	}

	d.dht = dhTable

	go func() {
		if err := dhTable.Bootstrap(d.ctx); err != nil {
			log.Errorf("dht: bootstrap: %s", err)
		}
	}()

	<-dhTable.RefreshRoutingTable()

	log.Infoln("dht: routing started")

	return dhTable, nil
}

func (d *DistributedHashTable) Close() {
	if d == nil || d.dht == nil {
		return
	}
	if err := d.dht.Close(); err != nil {
		log.Errorf("dht: table close: %v\n", err)
	}
	d.dht = nil
	log.Infoln("dht: table closed")
}
