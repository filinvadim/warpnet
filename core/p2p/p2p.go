package p2p

import (
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"time"
)

type NodeInfo struct {
	Addrs        []warpnet.WarpAddress `json:"addrs"`
	Protocols    []stream.WarpRoute    `json:"protocols"`
	Latency      time.Duration         `json:"latency"`
	PeerInfo     warpnet.WarpAddrInfo  `json:"peer"`
	NetworkState string                `json:"network_state"`
	ListenAddrs  []warpnet.WarpAddress `json:"listen_addrs"`
	Version      string                `json:"version"`
	StreamStats  network.Stats         `json:"stream_stats"`
	OwnerId      string                `json:"owner_id"`
}

const (
	DefaultTimeout = 60 * time.Second
	ServiceName    = "warpnet"
)

func NewP2PNode(
	privKey warpnet.WarpPrivateKey,
	store warpnet.WarpPeerstore,
	addrInfos []warpnet.PeerAddrInfo,
	conf config.Config,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
	rm network.ResourceManager,
	manager *connmgr.BasicConnMgr,
) (warpnet.P2PNode, error) {
	node, err := libp2p.New(
		libp2p.WithDialTimeout(DefaultTimeout),
		libp2p.ListenAddrStrings(conf.Node.ListenAddrs...),
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithConnectionTimeout(DefaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))), // TODO shuffle name. "warpnet" now
		libp2p.UserAgent(ServiceName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(addrInfos),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),
	)
	return node, err
}
