package handler

import (
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"net"
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

		fmt.Println(s)
		sent, recv := warpnet.GetNetworkIO()

		networkState := "Disconnected"
		peersOnline := i.Network().Peers()
		if len(peersOnline) != 0 {
			networkState = "Connected"
		}

		storedPeers := i.Peerstore().Peers()
		nodeInfo := i.NodeInfo()

		publicAddrs := make([]string, 0, len(nodeInfo.Addresses))
		for _, addr := range nodeInfo.Addresses {
			if !isPublicIP(addr) {
				continue
			}
			publicAddrs = append(publicAddrs, addr)
		}

		publicAddrsStr := "Waiting..."
		if len(publicAddrs) > 0 {
			publicAddrsStr = strings.Join(publicAddrs, ",")
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

func isPublicIP(addr string) bool {
	if addr == "" {
		return false
	}
	if strings.Contains(addr, "p2p-circuit") {
		return false
	}
	maddr, _ := warpnet.NewMultiaddr(addr)

	ipStr, err := maddr.ValueForProtocol(warpnet.P_IP4)
	if err != nil {
		ipStr, err = maddr.ValueForProtocol(warpnet.P_IP6)
		if err != nil {
			return false
		}
	}
	ip := net.ParseIP(ipStr)
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}

	// private ranges
	privateBlocks := []string{
		"10.0.0.0/8", // VPN
		"172.16.0.0/12",
		"192.168.0.0/16", // private network
		"100.64.0.0/10",  // CG-NAT
		"127.0.0.0/8",    // local
		"169.254.0.0/16", // link-local
	}

	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return false
		}
	}
	return true
}
