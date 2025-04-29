package warpnet

import (
	"context"
	"errors"
	"github.com/Masterminds/semver/v3"
	"github.com/docker/go-units"
	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/multiformats/go-multiaddr"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/net"
	log "github.com/sirupsen/logrus"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type NodeInfo struct {
	OwnerId   string          `json:"owner_id"`
	ID        WarpPeerID      `json:"node_id"`
	Version   *semver.Version `json:"version"`
	Addrs     AddrsInfo       `json:"addrs"`
	StartTime time.Time       `json:"start_time"`
}

type NodeStats struct {
	UserId  string          `json:"user_id"`
	NodeID  WarpPeerID      `json:"node_id"`
	Version *semver.Version `json:"version"`
	Addrs   AddrsInfo       `json:"addrs"`

	StartTime string `json:"start_time"`

	NetworkState string `json:"network_state"`

	DatabaseStats  map[string]string `json:"database_stats"`
	ConsensusStats map[string]string `json:"consensus_stats"`
	MemoryStats    map[string]string `json:"memory_stats"`
	CPUStats       map[string]string `json:"cpu_stats"`

	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`

	PeersOnline int `json:"peers_online"`
	PeersStored int `json:"peers_stored"`
}

func GetMacAddr() string {
	ifas, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr
		if a != "" {
			as = append(as, a)
		}
	}
	return strings.Join(as, ",")
}

func GetMemoryStats() map[string]string {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	return map[string]string{
		"heap":    units.HumanSize(float64(memStats.Alloc)),
		"stack":   units.HumanSize(float64(memStats.StackInuse)),
		"last_gc": time.Unix(0, int64(memStats.LastGC)).Format(time.DateTime),
	}
}

func GetCPUStats() map[string]string {
	cpuNum := strconv.Itoa(runtime.NumCPU())

	stats := map[string]string{
		"num": cpuNum,
	}

	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Error("could not get CPU usage percent", err)
		return stats
	}
	if len(percentages) == 0 {
		return stats
	}
	usage := strconv.FormatFloat(percentages[0], 'f', -1, 64)
	stats["usage"] = usage
	return stats
}

func GetNetworkIO() (bytesSent int64, bytesRecv int64) {
	ioCounters, err := net.IOCounters(false) // false = суммарно по всем интерфейсам
	if err != nil {
		log.Error("could not get network io counters", err)
		return 0, 0
	}
	if len(ioCounters) == 0 {
		return 0, 0
	}
	stats := ioCounters[0]
	return int64(stats.BytesSent), int64(stats.BytesRecv)
}

type AddrsInfo struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6,omitempty"`
}

var (
	ErrNodeIsOffline = errors.New("node is offline")
	ErrUserIsOffline = errors.New("user is offline")
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
