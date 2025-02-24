package handler

import (
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/warpnet"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

type NodeInformer interface {
	NodeInfo() p2p.NodeInfo
}

func StreamGetInfoHandler(i NodeInformer, handler discovery.DiscoveryHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() { s.Close() }() //#nosec

		handler(warpnet.PeerAddrInfo{
			ID:    s.Conn().RemotePeer(),
			Addrs: []warpnet.WarpAddress{s.Conn().RemoteMultiaddr()},
		})

		info := i.NodeInfo()
		info.StreamStats = s.Stat()

		if err := msgpack.NewEncoder(s).Encode(info); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}
		return
	}
}
