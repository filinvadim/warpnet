package warpnet

import (
	"context"
	"errors"
	"github.com/Masterminds/semver/v3"
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/multiformats/go-multiaddr"
)

type NodeInfo struct {
	ID             WarpPeerID        `json:"node_id"`
	Addrs          AddrsInfo         `json:"addrs"`
	NetworkState   string            `json:"network_state"`
	Version        *semver.Version   `json:"version"`
	OwnerId        string            `json:"owner_id"`
	PSK            pnet.PSK          `json:"psk"`
	PeersOnline    int               `json:"peers_online"`
	PeersStored    int               `json:"peers_stored"`
	DatabaseStats  map[string]string `json:"database_stats"`
	ConsensusStats map[string]string `json:"consensus_stats"`
	MemoryStats    map[string]string `json:"memory_stats"`
}

type AddrsInfo struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6,omitempty"`
}

var ErrNodeIsOffline = errors.New("node is offline")

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
	WarpProviderStore = providers.ProviderStore
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

func NewPeerstore(ctx context.Context, db datastore.Batching) (WarpPeerstore, error) {
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	return WarpPeerstore(store), err
}

type WarpRoutingFunc func(node P2PNode) (WarpPeerRouting, error)
