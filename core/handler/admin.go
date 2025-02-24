package handler

import (
	"github.com/filinvadim/warpnet/core/consensus"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

type AdminStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type AdminStateCommitter interface {
	CommitState(newState consensus.KVState) (_ *consensus.KVState, err error)
}

func StreamSelfHashVerifyHandler(state AdminStateCommitter) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		if state == nil {
			return nil, nil
		}
		var newState map[string]string
		err := msgpack.Unmarshal(buf, &newState)
		if err != nil {
			return nil, err
		}

		log.Infof("hash verif request received: %v", newState)

		updatedState, err := state.CommitState(newState)
		if err != nil {
			return nil, err
		}

		return updatedState, nil
	}
}
