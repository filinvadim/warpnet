package warpnet

import (
	"context"
	"errors"
	"github.com/Masterminds/semver/v3"
	"github.com/docker/go-units"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
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
	"github.com/libp2p/go-libp2p/core/transport"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/tcpreuse"

	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	tptu "github.com/libp2p/go-libp2p/p2p/net/upgrader"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/net"
	log "github.com/sirupsen/logrus"
	gonet "net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	BootstrapOwner = "bootstrap"
	WarpnetName    = "warpnet"
	NoiseID        = noise.ID

	Connected = network.Connected
	Limited   = network.Limited

	P_IP4 = multiaddr.P_IP4
	P_IP6 = multiaddr.P_IP6
	P_TCP = multiaddr.P_TCP

	PermanentAddrTTL = peerstore.PermanentAddrTTL
)

var (
	ErrNodeIsOffline = errors.New("node is offline")
	ErrUserIsOffline = errors.New("user is offline")

	privateBlocks = []string{
		"10.0.0.0/8", // VPN
		"172.16.0.0/12",
		"192.168.0.0/16", // private network
		"100.64.0.0/10",  // CG-NAT
		"127.0.0.0/8",    // local
		"169.254.0.0/16", // link-local
	}
)

// interfaces
type (
	WarpRelayCloser interface {
		Close() error
	}
	WarpGossiper interface {
		Close() error
	}
)

type (
	// types

	WarpMDNS        mdns.Service
	WarpPrivateKey  crypto.PrivKey
	WarpRoutingFunc func(node P2PNode) (WarpPeerRouting, error)

	// aliases
	TCPTransport       = tcp.TcpTransport
	TCPOption          = tcp.Option
	Swarm              = swarm.Swarm
	SwarmOption        = swarm.Option
	PSK                = pnet.PSK
	WarpProtocolID     = protocol.ID
	WarpStream         = network.Stream
	WarpStreamHandler  = network.StreamHandler
	WarpBatching       = datastore.Batching
	WarpProviderStore  = providers.ProviderStore
	PeerAddrInfo       = peer.AddrInfo
	WarpStreamStats    = network.Stats
	WarpPeerRouting    = routing.PeerRouting
	P2PNode            = host.Host
	WarpPeerstore      = peerstore.Peerstore
	WarpProtocolSwitch = protocol.Switch
	WarpNetwork        = network.Network
	WarpPeerID         = peer.ID
	WarpDHT            = dht.IpfsDHT
	WarpAddress        = multiaddr.Multiaddr
)

// structures
type WarpAddrInfo struct {
	ID    WarpPeerID `json:"peer_id"`
	Addrs []string   `json:"addrs"`
}

type NodeInfo struct {
	OwnerId        string          `json:"owner_id"`
	ID             WarpPeerID      `json:"node_id"`
	Version        *semver.Version `json:"version"`
	Addresses      []string        `json:"addresses"`
	StartTime      time.Time       `json:"start_time"`
	RelayState     string          `json:"relay_state"`
	BootstrapPeers []PeerAddrInfo  `json:"bootstrap_peers"`
}

func (ni NodeInfo) IsBootstrap() bool {
	return ni.OwnerId == BootstrapOwner
}

type NodeStats struct {
	UserId          string          `json:"user_id"`
	NodeID          WarpPeerID      `json:"node_id"`
	Version         *semver.Version `json:"version"`
	PublicAddresses string          `json:"public_addresses"`
	RelayState      string          `json:"relay_state"`

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

func NewP2PNode(opts ...libp2p.Option) (host.Host, error) {
	return libp2p.New(opts...)
}

func NewNoise(id protocol.ID, pk crypto.PrivKey, mxs []tptu.StreamMuxer) (*noise.Transport, error) {
	return noise.New(id, pk, mxs)
}

func NewTCPTransport(u transport.Upgrader, r network.ResourceManager, s *tcpreuse.ConnMgr, o ...tcp.Option) (*tcp.TcpTransport, error) {
	return tcp.NewTCPTransport(u, r, s, o...)
}

func NewConnManager(limiter rcmgr.Limiter) (*connmgr.BasicConnMgr, error) {
	return connmgr.NewConnManager(
		100,
		limiter.GetConnLimits().GetConnTotalLimit(),
		connmgr.WithGracePeriod(time.Hour*12),
	)
}

func NewResourceManager(limiter rcmgr.Limiter) (network.ResourceManager, error) {
	return rcmgr.NewResourceManager(limiter)
}

func NewAutoScaledLimiter() rcmgr.Limiter {
	defaultLimits := rcmgr.DefaultLimits.AutoScale()
	return rcmgr.NewFixedLimiter(defaultLimits)
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

func AddrInfoFromString(s string) (*PeerAddrInfo, error) {
	return peer.AddrInfoFromString(s)
}

func NewPeerstore(ctx context.Context, db datastore.Batching) (WarpPeerstore, error) {
	store, err := pstoreds.NewPeerstore(ctx, db, pstoreds.DefaultOpts())
	return WarpPeerstore(store), err
}

func IsPublicMultiAddress(maddr WarpAddress) bool {
	ipStr, err := maddr.ValueForProtocol(P_IP4)
	if err != nil {
		ipStr, err = maddr.ValueForProtocol(P_IP6)
		if err != nil {
			return false
		}
	}
	ip := gonet.ParseIP(ipStr)
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}

	for _, block := range privateBlocks {
		_, cidr, _ := gonet.ParseCIDR(block)
		if cidr.Contains(ip) {
			return false
		}
	}
	return true
}

func IsRelayAddress(addr string) bool {
	return strings.Contains(addr, "p2p-circuit")
}

func IsRelayMultiaddress(maddr multiaddr.Multiaddr) bool {
	return strings.Contains(maddr.String(), "p2p-circuit")
}
