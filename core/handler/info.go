package handler

import (
	"github.com/docker/go-units"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type NodeInformer interface {
	NodeInfo() warpnet.NodeInfo
}

type DBSizer interface {
	Size() int64
}

func StreamGetInfoHandler(i NodeInformer, db DBSizer, handler discovery.DiscoveryHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() { s.Close() }() //#nosec

		handler(warpnet.PeerAddrInfo{
			ID:    s.Conn().RemotePeer(),
			Addrs: []warpnet.WarpAddress{s.Conn().RemoteMultiaddr()},
		})

		info := i.NodeInfo()
		info.DatabaseSize = units.HumanSize(float64(db.Size()))

		if err := json.JSON.NewEncoder(s).Encode(info); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}
		return
	}
}
