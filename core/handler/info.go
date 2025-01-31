package handler

import (
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/p2p"
)

type NodeInformer interface {
	NodeInfo() p2p.NodeInfo
}

func StreamGetInfoHandler(i NodeInformer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		return i.NodeInfo(), nil
	}
}
