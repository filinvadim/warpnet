package handler

import (
	"github.com/docker/go-units"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"runtime"
	"time"
)

type NodeInformer interface {
	NodeInfo() warpnet.NodeInfo
}

type DBSizer interface {
	Stats() map[string]string
}

type ConsensusStatsProvider interface {
	Stats() map[string]string
}

func StreamGetInfoHandler(
	i NodeInformer,
	db DBSizer,
	consensus ConsensusStatsProvider,
	handler discovery.DiscoveryHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() { s.Close() }() //#nosec

		handler(warpnet.PeerAddrInfo{
			ID:    s.Conn().RemotePeer(),
			Addrs: []warpnet.WarpAddress{s.Conn().RemoteMultiaddr()},
		})

		memStats := runtime.MemStats{}
		runtime.ReadMemStats(&memStats)

		info := i.NodeInfo()
		info.DatabaseStats = db.Stats()
		info.ConsensusStats = consensus.Stats()
		info.MemoryStats = map[string]string{
			"heap":    units.HumanSize(float64(memStats.Alloc)),
			"stack":   units.HumanSize(float64(memStats.StackInuse)),
			"last_gc": time.Unix(0, int64(memStats.LastGC)).Format(time.DateTime),
		}

		if err := json.JSON.NewEncoder(s).Encode(info); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}
		return
	}
}
