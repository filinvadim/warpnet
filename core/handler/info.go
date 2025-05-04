package handler

import (
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type NodeInformer interface {
	NodeInfo() warpnet.NodeInfo
}

func StreamGetInfoHandler(
	i NodeInformer,
	handler discovery.DiscoveryHandler,
) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() { s.Close() }() //#nosec

		remoteAddr := s.Conn().RemoteMultiaddr()
		s.Conn().RemotePeer().String()

		log.Infof("node info request received: %s %s", s.Conn().RemotePeer().String(), remoteAddr)

		if handler != nil {
			handler(warpnet.PeerAddrInfo{
				ID:    s.Conn().RemotePeer(),
				Addrs: []warpnet.WarpAddress{remoteAddr},
			})
		}

		info := i.NodeInfo()
		info.RequesterAddr = remoteAddr.String()

		if err := json.JSON.NewEncoder(s).Encode(info); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}

		log.Infof("node info response sent: %v", info)

		return
	}
}
