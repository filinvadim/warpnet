package handler

import (
	"github.com/filinvadim/warpnet/core/consensus"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type AdminStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type AdminStateCommitter interface {
	CommitState(newState consensus.KVState) (_ *consensus.KVState, err error)
}

type ConsensusResetter interface {
	Reset() error
}

func StreamVerifyHandler(state AdminStateCommitter) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		if state == nil {
			return nil, nil
		}
		var newState map[string]string
		err := json.JSON.Unmarshal(buf, &newState)
		if err != nil {
			return nil, err
		}

		log.Infof("node verify request received: %v", newState)

		updatedState, err := state.CommitState(newState)
		if err != nil {
			return nil, err
		}

		return updatedState, nil
	}
}

func StreamConsensusResetHandler(consRepo ConsensusResetter) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		if consRepo == nil {
			return nil, nil
		}

		return event.Accepted, consRepo.Reset()
	}
}
