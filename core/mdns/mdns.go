package mdns

import (
	"context"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

/*
  Multicast DNS (mDNS) is a protocol that allows devices to discover each other on a local network
  without the need for centralized servers. It works by sending multicast DNS queries,
  enabling nodes to find other nodes by hostname.

  ### **mDNS is used in:**
  - Apple Bonjour (automatic device discovery in networks)
  - Google Chromecast, AirPlay
  - IoT devices and P2P networks
  - Decentralized systems where no central server is available for node announcements

  The **go-libp2p-mdns** library is an implementation of mDNS for libp2p, designed for
  automatic peer discovery in local networks without requiring a centralized server.

  ### **Key Features of go-libp2p-mdns:**
  - **Local Peer Discovery**
    - Enables automatic detection of network participants without manual configuration.
  - **Works via UDP Multicast Queries**
    - Allows DNS queries to be transmitted without using a traditional DNS server.
  - **Flexibility**
    - Suitable for temporary (Ad-Hoc) networks and local P2P systems.
  - **Zero-Configuration**
    - Simplifies the deployment of P2P applications without requiring manual node address setup.
  - **Optimized for Small Networks**
    - Ideal for local applications but inefficient for large-scale global P2P networks.
*/

const mdnsServiceName = "warpnet"

type NodeConnector interface {
	Connect(peer.AddrInfo) error
	Node() warpnet.P2PNode
}

type MulticastDNS struct {
	mdns      warpnet.WarpMDNS
	service   *mdnsDiscoveryService
	isRunning *atomic.Bool
}

type mdnsDiscoveryService struct {
	ctx               context.Context
	discoveryHandlers []discovery.DiscoveryHandler
	node              NodeConnector
	mx                *sync.Mutex
}

func (m *mdnsDiscoveryService) HandlePeerFound(p peer.AddrInfo) {
	if m == nil {
		return
	}
	if m.node == nil {
		panic("mdns: node is nil")
	}

	log.Debugf("mdns: discovery handling peer %s %v", p.ID.String(), p.Addrs)

	m.mx.Lock()
	defer m.mx.Unlock()

	if m.discoveryHandlers != nil {
		for _, h := range m.discoveryHandlers {
			h(p)
		}
		return
	}
	m.defaultDiscoveryHandler(p)
}

func (m *mdnsDiscoveryService) defaultDiscoveryHandler(peerInfo warpnet.PeerAddrInfo) {
	if err := m.node.Connect(peerInfo); err != nil {
		log.Errorf(
			"mdns: discovery: failed to connect to peer %s: %v",
			peerInfo.String(),
			err,
		)
		return
	}
	log.Debugf("mdns: discovery: connected to peer: %s %s", peerInfo.Addrs, peerInfo.ID)
	return
}

func NewMulticastDNS(ctx context.Context, discoveryHandlers ...discovery.DiscoveryHandler) *MulticastDNS {
	service := &mdnsDiscoveryService{
		ctx:               ctx,
		discoveryHandlers: discoveryHandlers,
		node:              nil,
		mx:                new(sync.Mutex),
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

	m.service.mx.Lock()
	m.service.node = n
	m.service.mx.Unlock()

	m.mdns = mdns.NewMdnsService(n.Node(), mdnsServiceName, m.service)

	// start it a little bit later,
	// wait and try trivial discovery first,
	// MDNS discovery might hide potential bugs
	time.Sleep(time.Minute * 5)

	if err := m.mdns.Start(); err != nil {
		log.Errorf("mdns: failed to start: %v", err)
		return
	}
	log.Infoln("mdns: service started")
}

func (m *MulticastDNS) Close() {
	if m == nil || m.mdns == nil {
		return
	}
	if !m.isRunning.Load() {
		return
	}
	if err := m.mdns.Close(); err != nil {
		log.Errorf("mdns: failed to close: %v", err)
	}
	m.isRunning.Store(false)
	m.mdns = nil
}
