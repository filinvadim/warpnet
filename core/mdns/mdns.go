package mdns

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"log"
	"sync"
	"sync/atomic"
)

/*
  Multicast DNS (mDNS) — это протокол, позволяющий устройствам обнаруживать друг друга в локальной сети
без использования централизованных серверов. Он работает путём многоадресной рассылки (multicast)
DNS-запросов, что позволяет узлам находить другие узлы по именам хостов.
  mDNS используется в:
- Apple Bonjour (автоматическое обнаружение устройств в сети)
- Google Chromecast, AirPlay
- IoT-устройствах и P2P-сетях
- Децентрализованных системах, где нет центрального сервера для объявления узлов

  Библиотекаgo-libp2p-mdns — это реализация mDNS для libp2p, предназначенная для автоматического обнаружения
узлов в локальной сети без необходимости централизованного сервера.
  Основные характеристики go-libp2p-mdns:
- Обнаружение узлов в локальной сети
- Позволяет автоматически находить других участников сети без ручной конфигурации.
- Работает через UDP-многоадресные запросы
- Позволяет передавать DNS-запросы без использования традиционного DNS-сервера.
- Гибкость
- Подходит для временных сетей (Ad-Hoc) и локальных P2P-систем.
- Zero-configuration
- Упрощает запуск P2P-приложений без необходимости ручного указания адресов узлов.
- Оптимизирован для небольших сетей
- Подходит для локальных приложений, но неэффективен в глобальных масштабируемых P2P-сетях.
*/

const mdnsServiceName = "warpnet"

type NodeConnector interface {
	Connect(peer.AddrInfo) error
	Node() warpnet.P2PNode
}

type MulticastDNS struct {
	mdns    warpnet.WarpMDNS
	service *mdnsDiscoveryService

	isRunning *atomic.Bool
}

type mdnsDiscoveryService struct {
	ctx              context.Context
	discoveryHandler discovery.DiscoveryHandler
	node             NodeConnector
	mx               *sync.Mutex
}

func (m *mdnsDiscoveryService) joinNode(n NodeConnector) {
	if m == nil {
		return
	}
	m.mx.Lock()
	defer m.mx.Unlock()
	m.node = n
}

func (m *mdnsDiscoveryService) HandlePeerFound(p peer.AddrInfo) {
	if m == nil {
		return
	}
	if m.node == nil {
		panic("mdns: node is nil")
	}
	fmt.Println("mdns: handle peer found", p.ID)
	m.mx.Lock()
	defer m.mx.Unlock()
	if m.discoveryHandler == nil {
		if err := m.node.Connect(p); err != nil {
			log.Println("mdns: failed to connect to peer", p.ID, err)
		}
		log.Println("mdns: connected to peer:", p.ID)
		return
	}
	m.discoveryHandler(p)
}

func NewMulticastDNS(ctx context.Context, discoveryHandler discovery.DiscoveryHandler) *MulticastDNS {
	service := &mdnsDiscoveryService{
		ctx:              ctx,
		discoveryHandler: discoveryHandler,
		node:             nil,
		mx:               new(sync.Mutex),
	}
	return &MulticastDNS{nil, service, new(atomic.Bool)}
}

func (m *MulticastDNS) Start(n NodeConnector) {
	if m == nil {
		return
	}
	if m.isRunning.Load() {
		return
	}
	m.isRunning.Store(true)

	m.service.joinNode(n)

	m.mdns = mdns.NewMdnsService(n.Node(), mdnsServiceName, m.service)

	if err := m.mdns.Start(); err != nil {
		log.Println("mdns failed to start", err)
		return
	}
	log.Println("mdns service started")
}

func (m *MulticastDNS) Close() {
	if m == nil || m.mdns == nil {
		return
	}
	if !m.isRunning.Load() {
		return
	}
	if err := m.mdns.Close(); err != nil {
		log.Println("mdns failed to close", err)
	}
	m.isRunning.Store(false)
}
