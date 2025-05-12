/*

Warpnet - Decentralized Social Network
Copyright (C) 2025 Vadim Filin, https://github.com/filinvadim,
<github.com.mecdy@passmail.net>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Copyright 2025 Vadim Filin
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
