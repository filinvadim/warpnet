package node

import (
	"context"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
)

// 2025-01-25T19:54:43.327+0400	WARN
//dht	go-libp2p-kad-dht/dht.go:523
//failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
//"error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
//all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
//peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
//but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}

func setupPrivateDHT(
	ctx context.Context,
	n types.P2PNode,
	repo datastore.Batching,
	providerStore providers.ProviderStore,
	relays []types.PeerAddrInfo,

) (*types.WarpDHT, error) {
	DHT, err := dht.New(
		ctx, n,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(ProtocolPrefix),
		dht.Datastore(repo),
		dht.BootstrapPeers(relays...),
		dht.ProviderStore(providerStore),
	)
	if err != nil {
		log.Printf("new dht: %v\n", err)
		return nil, err
	}
	DHT.RoutingTable().PeerRemoved = func(id peer.ID) {
		log.Println("DHT removed peer", id)
	}
	DHT.RoutingTable().PeerAdded = func(id peer.ID) {
		log.Println("DHT added peer", id)
	}

	go func() {
		if err := DHT.Bootstrap(ctx); err != nil {
			log.Printf("DHT bootstrap: %s", err)
		}
	}()

	if err = <-DHT.RefreshRoutingTable(); err != nil {
		log.Printf("failed to refresh kdht routing table: %v\n", err)
	}

	return DHT, nil
}
