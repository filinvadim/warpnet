package p2p

import (
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/security"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"time"
)

type NodeInfo struct {
	Addrs        []string        `json:"addrs"`
	Latency      time.Duration   `json:"latency"`
	NetworkState string          `json:"network_state"`
	Version      *semver.Version `json:"version"`
	StreamStats  network.Stats   `json:"stream_stats"`
	OwnerId      string          `json:"owner_id"`
	SelfHash     string          `json:"self_hash"`
	Protocols    []string        `json:"protocols"`
}

const (
	DefaultTimeout = 180 * time.Second
	ServiceName    = "warpnet"
)

func NewP2PNode(
	privKey warpnet.WarpPrivateKey,
	store warpnet.WarpPeerstore,
	listenAddr string,
	conf config.Config,
	routingFn func(node warpnet.P2PNode) (warpnet.WarpPeerRouting, error),
) (warpnet.P2PNode, error) {
	limiter := rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale())

	manager, err := connmgr.NewConnManager(
		100,
		limiter.GetConnLimits().GetConnTotalLimit(),
		connmgr.WithGracePeriod(time.Hour*12),
	)
	if err != nil {
		return nil, err
	}

	peersList := store.PeersWithAddrs()
	staticRelaysList := make([]warpnet.PeerAddrInfo, 0, len(peersList))
	for _, p := range peersList {
		info := store.PeerInfo(p)
		staticRelaysList = append(staticRelaysList, info)
	}

	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	return libp2p.New(
		libp2p.WithDialTimeout(DefaultTimeout),
		libp2p.ListenAddrStrings(
			listenAddr,
		),
		libp2p.SwarmOpts(
			swarm.WithDialTimeout(DefaultTimeout),
			swarm.WithDialTimeoutLocal(DefaultTimeout),
		),
		libp2p.Transport(tcp.NewTCPTransport, tcp.WithConnectionTimeout(DefaultTimeout)),
		libp2p.Identity(privKey),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPublic(), // TODO
		libp2p.PrivateNetwork(security.ConvertToSHA256([]byte(conf.Node.Prefix))), // TODO shuffle name thru consensus. "warpnet" now
		libp2p.UserAgent(ServiceName),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.ResourceManager(rm),
		libp2p.EnableRelayService(relayv2.WithInfiniteLimits()),
		libp2p.EnableAutoRelayWithStaticRelays(staticRelaysList),
		libp2p.ConnectionManager(manager),
		libp2p.Routing(routingFn),
	)
}
