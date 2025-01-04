package types

import (
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

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

type (
	WarpPeerRouting   = routing.PeerRouting
	P2PNode           = host.Host
	WarpAddrInfo      = peer.AddrInfo
	WarpPeerstore     = peerstore.Peerstore
	WarpPrivateKey    crypto.PrivKey
	WarpDiscriminator = protocol.ID
	WarpPeerID        = peer.ID
	WarpDHT           = dht.IpfsDHT
)
