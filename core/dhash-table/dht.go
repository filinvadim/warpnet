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
	"io"
	"log"
	"time"
)

const ProtocolPrefix = "/warpnet"

// TODO: track this: dht	go-libp2p-kad-dht/dht.go:523
//failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
//"error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
//all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
//peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
//but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}

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
}

func DefaultNodeRemovedCallback(info warpnet.PeerAddrInfo) {
	log.Println("dht: node removed", info.ID)
}

func DefaultNodeAddedCallback(info warpnet.PeerAddrInfo) {
	log.Println("dht: node added", info.ID)
}

func NewDHTable(
	ctx context.Context,
	db RoutingStorer,
	providerStore ProviderStorer,
	conf config.Config,
	addF discovery.DiscoveryHandler,
	removeF discovery.DiscoveryHandler,
) *DistributedHashTable {
	bootstrapAddrs, _ := conf.Node.AddrInfos()
	return &DistributedHashTable{
		ctx:           ctx,
		db:            db,
		providerStore: providerStore,
		boostrapNodes: bootstrapAddrs,
		addF:          addF,
		removeF:       removeF,
	}
}

func (d *DistributedHashTable) StartRouting(n warpnet.P2PNode) (_ warpnet.WarpPeerRouting, err error) {
	dhTable, err := dht.New(
		d.ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(ProtocolPrefix),
		dht.Datastore(d.db),
		dht.BootstrapPeers(d.boostrapNodes...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Minute*5),
	)
	if err != nil {
		log.Printf("new dht: %v\n", err)
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
			log.Printf("dht: bootstrap: %s", err)
		}
	}()

	<-dhTable.RefreshRoutingTable()

	log.Println("dht: routing started")

	return dhTable, nil
}

func (d *DistributedHashTable) Close() {
	if d == nil || d.dht == nil {
		return
	}
	if err := d.dht.Close(); err != nil {
		log.Printf("dht: table close: %v\n", err)
	}
}
