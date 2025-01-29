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

/*
  Distributed Hash Table (DHT) — это распределённая хеш-таблица, используемая для децентрализованного
хранения и поиска данных в одноранговых (P2P) сетях. Вместо того чтобы хранить данные на одном сервере,
DHT распределяет их между множеством узлов.
DHT решает три основные задачи:
1. Маршрутизация — позволяет эффективно находить узлы, хранящие определённые ключи.
2. Хранение данных — каждый узел отвечает за часть пространства ключей.
3. Поиск по ключу — обеспечивает быстрый доступ к данным без центрального сервера.
  DHT используется в BitTorrent, IPFS, Ethereum, а также в P2P-мессенджерах и других децентрализованных
приложениях.

  Библиотека go-libp2p-kad-dht — это реализация Kademlia DHT для libp2p. Она позволяет одноранговым узлам
обмениваться данными и находить друг друга без централизованных серверов.
Основные характеристики go-libp2p-kad-dht:
- Алгоритм Kademlia
- Использует Kademlia DHT, один из самых популярных алгоритмов для распределённых таблиц.
- Поиск узлов и данных в P2P-сети
- Позволяет находить ноды и запрашивать у них данные по ключу.
- Гибкая маршрутизация
- Оптимизирован для работы в сетях с высокой динамикой (узлы могут подключаться и отключаться).
- Поддержка PubSub и IPFS
- Используется в IPFS и может применяться в P2P-мессенджерах и децентрализованных приложениях.
- Хэширование ключей
- Разбивает пространство ключей по узлам, что позволяет равномерно распределять нагрузку.

DHT подходит для децентрализованных приложений, которым нужен распределённый поиск без единой точки отказа,
P2P-сетей, где узлы постоянно подключаются и отключаются, обмена данными между нодами без централизованного сервера.
  Библиотека go-libp2p-kad-dht удобен, если требуется найти другие узлы в libp2p-сети, нужно организовать
децентрализованный поиск контента (как в IPFS), требуется эффективная маршрутизация в распределённой сети.
*/

const protocolPrefix = "/warpnet"

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

// TODO: track this: dht	go-libp2p-kad-dht/dht.go:523
// failed to bootstrap	{"peer": "12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo",
// "error": "failed to dial: failed to dial 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo:
// all dials failed\n  * [/ip4/67.207.72.168/tcp/4001] failed to negotiate security protocol:
// peer id mismatch: expected 12D3KooWJAYu4meUU7v5usd7P4b5LAJjBH6svwmGZqoVe24rLEQo,
// but remote key matches 12D3KooWSmiUppeMgcxGgPzJheaDfQvGuUpa9JzciDfpMea2epG3"}
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
		dht.ProtocolPrefix(protocolPrefix),
		dht.Datastore(d.db),
		dht.MaxRecordAge(time.Hour*24*365),
		dht.RoutingTableRefreshPeriod(time.Hour*24),
		dht.RoutingTableRefreshQueryTimeout(time.Hour*24),
		dht.BootstrapPeers(d.boostrapNodes...),
		dht.ProviderStore(d.providerStore),
		dht.RoutingTableLatencyTolerance(time.Hour*24),
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
