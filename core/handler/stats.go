// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

package handler

import (
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"strings"
	"time"
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
		sent, recv := warpnet.GetNetworkIO()

		networkState := "Disconnected"
		peersOnline := i.Network().Peers()
		if len(peersOnline) != 0 {
			networkState = "Connected"
		}

		storedPeers := i.Peerstore().Peers()
		nodeInfo := i.NodeInfo()

		publicAddrsStr := "Waiting..."
		if len(nodeInfo.Addresses) > 0 {
			publicAddrsStr = strings.Join(nodeInfo.Addresses, ",")
		}

		stats := warpnet.NodeStats{
			UserId:          nodeInfo.OwnerId,
			NodeID:          nodeInfo.ID,
			Version:         nodeInfo.Version,
			PublicAddresses: publicAddrsStr,
			StartTime:       nodeInfo.StartTime.Format(time.DateTime),
			NetworkState:    networkState,
			RelayState:      nodeInfo.RelayState,
			DatabaseStats:   db.Stats(),
			ConsensusStats:  consensus.Stats(),
			MemoryStats:     warpnet.GetMemoryStats(),
			CPUStats:        warpnet.GetCPUStats(),
			BytesSent:       sent,
			BytesReceived:   recv,
			PeersOnline:     len(peersOnline),
			PeersStored:     len(storedPeers),
		}
		return stats, nil
	}
}
