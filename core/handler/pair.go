package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

func StreamNodesPairingHandler(serverAuthInfo domain.AuthNodeInfo) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var clientInfo domain.AuthNodeInfo
		if err := msgpack.Unmarshal(buf, &clientInfo); err != nil {
			log.Errorf("pair: unmarshaling from stream: %v", err)
			return nil, err
		}
		tokenMatch := serverAuthInfo.Identity.Token == clientInfo.Identity.Token
		if !tokenMatch {
			log.Errorf("pair: token does not match server identity")
			return nil, errors.New("token mismatch")
		}
		return nil, nil
	}
}
