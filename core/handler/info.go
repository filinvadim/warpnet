package handler

import (
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/warpnet"
)

type NodeInformer interface {
	NodeInfo(s warpnet.WarpStream) p2p.NodeInfo
}

func StreamGetInfoHandler(mr middleware.MiddlewareResolver, i NodeInformer) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		mr.UnwrapStream(s, func(buf []byte) (any, error) {
			return i.NodeInfo(s), nil
		})
	}
}
