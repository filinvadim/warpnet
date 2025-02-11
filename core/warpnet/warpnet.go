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

	WarpPeerstore      = peerstore.Peerstore
	WarpProtocolSwitch = protocol.Switch
	WarpNetwork        = network.Network
	WarpPeerID         = peer.ID
	WarpDHT            = dht.IpfsDHT
	WarpAddress        = multiaddr.Multiaddr
)

func FromStringToPeerID(s string) WarpPeerID {
	peerID, err := peer.Decode(s)
	if err != nil {
		return ""
	}
	return peerID
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
