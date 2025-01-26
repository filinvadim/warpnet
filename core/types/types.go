package types

import (
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/gen/domain-gen"
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
	"time"
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

type WarpAddrInfo struct {
	ID    WarpPeerID `json:"peer_id"`
	Addrs []string   `json:"addrs"`
}

type (
	PeerAddrInfo    = peer.AddrInfo
	WarpStreamStats = network.Stats
	WarpPeerRouting = routing.PeerRouting
	P2PNode         = host.Host

	WarpPeerstore     = peerstore.Peerstore
	WarpPrivateKey    crypto.PrivKey
	WarpDiscriminator = protocol.ID
	WarpPeerID        = peer.ID
	WarpDHT           = dht.IpfsDHT
	WarpAddress       = multiaddr.Multiaddr
)

type NodeInfo struct {
	Addrs        []WarpAddress       `json:"addrs"`
	Protocols    []WarpDiscriminator `json:"protocols"`
	Latency      time.Duration       `json:"latency"`
	PeerInfo     WarpAddrInfo        `json:"peer"`
	NetworkState string              `json:"network_state"`
	ListenAddrs  []WarpAddress       `json:"listen_addrs"`
	Owner        domain.Owner        `json:"owner"`
	Version      *semver.Version     `json:"version"`
	StreamStats  network.Stats       `json:"stream_stats"`
}
