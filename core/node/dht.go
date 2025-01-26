package node

import (
	"context"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"time"
)

// TODO: track this: dht	go-libp2p-kad-dht/dht.go:523
//failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
//"error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
//all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
//peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
//but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}

type DistributedHashTable struct {
	ctx           context.Context
	batchingRepo  datastore.Batching
	providerStore providers.ProviderStore
	relays        []types.PeerAddrInfo
	addF          DiscoveryHandler
	removeF       DiscoveryHandler
}

func NewDHTable(
	ctx context.Context,
	batchingRepo datastore.Batching,
	providerStore providers.ProviderStore,
	relays []types.PeerAddrInfo,
	addF DiscoveryHandler,
	removeF DiscoveryHandler,
) *DistributedHashTable {
	return &DistributedHashTable{
		ctx:           ctx,
		batchingRepo:  batchingRepo,
		providerStore: providerStore,
		relays:        relays,
		addF:          addF,
		removeF:       removeF,
	}
}

func (d *DistributedHashTable) Start(n types.P2PNode) (_ types.WarpPeerRouting, err error) {
	dhTable, err := dht.New(
		d.ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(ProtocolPrefix),
		dht.Datastore(d.batchingRepo),
		dht.BootstrapPeers(d.relays...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Hour*24), // if one day node is not responding
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

	go func() {
		if err := dhTable.Bootstrap(d.ctx); err != nil {
			log.Printf("DHT bootstrap: %s", err)
		}
	}()

	<-dhTable.RefreshRoutingTable()

	return dhTable, nil
}
