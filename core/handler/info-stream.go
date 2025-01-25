package handler

import (
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/gen/domain-gen"

	"github.com/filinvadim/warpnet/core/types"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

type NodeInfoStorer interface {
	ID() types.WarpPeerID
	Peerstore() peerstore.Peerstore
	Addrs() []types.WarpAddress
	Network() network.Network
}

func StreamGetInfoHandler(
	node NodeInfoStorer,
	owner domain.Owner,
	version *semver.Version,
) func(s network.Stream) {
	return func(s network.Stream) {
		ReadStream(s, func(buf []byte) (any, error) {
			id := node.ID()
			addrs := node.Peerstore().Addrs(id)
			protocols, _ := node.Peerstore().GetProtocols(id)
			latency := node.Peerstore().LatencyEWMA(id)
			peerInfo := node.Peerstore().PeerInfo(id)
			connectedness := node.Network().Connectedness(id)
			listenAddrs := node.Network().ListenAddresses()

			plainAddrs := make([]string, 0, len(peerInfo.Addrs))
			for _, a := range peerInfo.Addrs {
				plainAddrs = append(plainAddrs, a.String())
			}

			var nodeInfo = types.NodeInfo{
				Addrs:     addrs,
				Protocols: protocols,
				Latency:   latency,
				PeerInfo: types.WarpAddrInfo{
					ID:    peerInfo.ID,
					Addrs: plainAddrs,
				},
				NetworkState: connectedness.String(),
				ListenAddrs:  listenAddrs,
				Owner:        owner,
				Version:      version,
				StreamStats:  s.Stat(),
			}

			return nodeInfo, nil
		})
	}
}
