// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

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

		log.Debugf("node info request received: %s %s", s.Conn().RemotePeer().String(), remoteAddr)

		if handler != nil {
			handler(warpnet.PeerAddrInfo{
				ID:    s.Conn().RemotePeer(),
				Addrs: []warpnet.WarpAddress{remoteAddr},
			})
		}

		if err := json.JSON.NewEncoder(s).Encode(i.NodeInfo()); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}
		return
	}
}
