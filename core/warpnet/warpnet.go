package warpnet

import (
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"

	"strings"
	"time"
)

const PermanentAddrTTL = peerstore.PermanentAddrTTL

type (
	WarpRelayCloser interface {
		Close() error
	}
	WarpGossiper interface {
		Close() error
	}
)

type (
	WarpMDNS mdns.Service
)

type WarpAddrInfo struct {
	ID    WarpPeerID `json:"peer_id"`
	Addrs []string   `json:"addrs"`
}

type (
	WarpRoute  string
	WarpRoutes []WarpRoute
)

func (r WarpRoute) ProtocolID() protocol.ID {
	return protocol.ID(r)
}

func (r WarpRoute) String() string {
	return string(r)
}

func (r WarpRoute) IsPrivate() bool {
	return strings.Contains(string(r), "private")
}

func (rs WarpRoutes) FromRoutesToPrIDs() []protocol.ID {
	prIDs := make([]protocol.ID, 0, len(rs))
	for _, r := range rs {
		prIDs = append(prIDs, r.ProtocolID())
	}
	return prIDs
}

func FromPrIDToRoute(prID protocol.ID) WarpRoute {
	return WarpRoute(prID)
}

func FromPrIDToRoutes(prIDs []protocol.ID) WarpRoutes {
	rs := make(WarpRoutes, 0, len(prIDs))
	for _, p := range prIDs {
		rs = append(rs, WarpRoute(p))
	}
	return rs
}

func IDFromBytes(b []byte) (WarpPeerID, error) {
	if _, err := mh.Cast(b); err != nil {
		return WarpPeerID(""), err
	}
	return WarpPeerID(b), nil
}

type WarpPrivateKey crypto.PrivKey

type (
	WarpProtocolID    = protocol.ID
	WarpStream        = network.Stream
	WarpStreamHandler = network.StreamHandler
	WarpBatching      = datastore.Batching
	PeerAddrInfo      = peer.AddrInfo
	WarpStreamStats   = network.Stats
	WarpPeerRouting   = routing.PeerRouting
	P2PNode           = host.Host

	WarpPeerstore = peerstore.Peerstore
	WarpNetwork   = network.Network
	WarpPeerID    = peer.ID
	WarpDHT       = dht.IpfsDHT
	WarpAddress   = multiaddr.Multiaddr
)

type NodeInfo struct {
	Addrs        []WarpAddress `json:"addrs"`
	Protocols    []WarpRoute   `json:"protocols"`
	Latency      time.Duration `json:"latency"`
	PeerInfo     WarpAddrInfo  `json:"peer"`
	NetworkState string        `json:"network_state"`
	ListenAddrs  []WarpAddress `json:"listen_addrs"`
	Version      string        `json:"version"`
	StreamStats  network.Stats `json:"stream_stats"`
	OwnerId      string        `json:"owner_id"`
}

func NewMultiaddr(s string) (a multiaddr.Multiaddr, err error) {
	return multiaddr.NewMultiaddr(s)
}

func IDFromPrivateKey(sk crypto.PrivKey) (WarpPeerID, error) {
	return peer.IDFromPublicKey(sk.GetPublic())
}

func AddrInfoFromP2pAddr(m multiaddr.Multiaddr) (*PeerAddrInfo, error) {
	return peer.AddrInfoFromP2pAddr(m)
}
