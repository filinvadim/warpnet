package handler

import (
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
)

type StatsNodeInformer interface {
	NodeInfo() warpnet.NodeInfo
	Peerstore() warpnet.WarpPeerstore
	Network() warpnet.WarpNetwork
}

type StatsProvider interface {
	Stats() map[string]string
}

func StreamGetStatsHandler(
	i StatsNodeInformer,
	db StatsProvider,
	consensus StatsProvider,
) middleware.WarpHandler {
	return func(_ []byte, s warpnet.WarpStream) (any, error) {
		defer func() { s.Close() }() //#nosec

		sent, recv := warpnet.GetNetworkIO()

		networkState := "Disconnected"
		peersOnline := i.Network().Peers()
		if len(peersOnline) != 0 {
			networkState = "Connected"
		}

		storedPeers := i.Peerstore().Peers()

		nodeInfo := i.NodeInfo()
		stats := warpnet.NodeStats{
			UserId:         nodeInfo.OwnerId,
			NodeID:         nodeInfo.ID,
			Version:        nodeInfo.Version,
			Addrs:          nodeInfo.Addrs,
			StartTime:      nodeInfo.StartTime,
			NetworkState:   networkState,
			DatabaseStats:  db.Stats(),
			ConsensusStats: consensus.Stats(),
			MemoryStats:    warpnet.GetMemoryStats(),
			CPUStats:       warpnet.GetCPUStats(),
			BytesSent:      sent,
			BytesReceived:  recv,
			PeersOnline:    len(peersOnline),
			PeersStored:    len(storedPeers),
		}
		return stats, nil
	}
}
