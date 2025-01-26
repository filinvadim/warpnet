package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"log"
	"sync"
	"sync/atomic"
)

const mdnsServiceName = "warpnet"

type NodeConnector interface {
	Connect(context.Context, peer.AddrInfo) error
}

type MulticastDNS struct {
	mdns    types.WarpMDNS
	service *mdnsDiscoveryService

	isRunning *atomic.Bool
}

type mdnsDiscoveryService struct {
	ctx              context.Context
	discoveryHandler DiscoveryHandler
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
		if err := m.node.Connect(m.ctx, p); err != nil {
			log.Println("mdns: failed to connect to peer", p.ID, err)
		}
		log.Println("mdns: connected to peer:", p.ID)
		return
	}
	m.discoveryHandler(p)
}

func NewMulticastDNS(ctx context.Context, discoveryHandler DiscoveryHandler) *MulticastDNS {
	service := &mdnsDiscoveryService{
		ctx:              ctx,
		discoveryHandler: discoveryHandler,
		node:             nil,
		mx:               new(sync.Mutex),
	}
	return &MulticastDNS{nil, service, new(atomic.Bool)}
}

func (m *MulticastDNS) Start(n *WarpNode) {
	if m == nil {
		return
	}
	if m.isRunning.Load() {
		return
	}
	m.isRunning.Store(true)
	
	m.service.joinNode(n.Node())

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
