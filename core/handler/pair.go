package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
)

func StreamNodesPairingHandler(serverAuthInfo domain.AuthNodeInfo) func(buf []byte) (any, error) {
	return func(buf []byte) (any, error) {
		var clientInfo domain.AuthNodeInfo
		if err := json.JSON.Unmarshal(buf, &clientInfo); err != nil {
			log.Printf("bind: unmarshaling from stream: %v", err)
			return nil, err
		}
		tokenMatch := serverAuthInfo.Identity.Token == clientInfo.Identity.Token
		if !tokenMatch {
			return nil, errors.New("token mismatch")
		}
		return nil, nil
	}
}
