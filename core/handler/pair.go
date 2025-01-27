package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	warpnet "github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
)

func StreamNodesPairingHandler(mr middleware.MiddlewareResolver, serverAuthInfo domain.AuthNodeInfo) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		pairF := func(buf []byte) (any, error) {
			var clientInfo domain.AuthNodeInfo
			if err := json.JSON.Unmarshal(buf, &clientInfo); err != nil {
				log.Printf("bind: unmarshaling from stream: %v", err)
				return nil, err
			}
			tokenMatch := serverAuthInfo.Identity.Token == clientInfo.Identity.Token
			if !tokenMatch {
				log.Printf("stream: token or peer ID mismatch for %s %s", s.Protocol(), s.Conn().RemotePeer())
				return nil, errors.New("token mismatch")
			}
			mr.SetClientID(s.Conn().RemotePeer())

			return nil, nil
		}

		mr.UnwrapStream(s, pairF)
	}
}
