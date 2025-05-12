// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

package handler

import (
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

func StreamNodesPairingHandler(serverAuthInfo domain.AuthNodeInfo) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var clientInfo domain.AuthNodeInfo
		if err := json.JSON.Unmarshal(buf, &clientInfo); err != nil {
			log.Errorf("pair: unmarshaling from stream: %v", err)
			return nil, err
		}
		tokenMatch := serverAuthInfo.Identity.Token == clientInfo.Identity.Token
		if !tokenMatch {
			log.Errorf("pair: token does not match server identity")
			return nil, warpnet.WarpError("token mismatch")
		}
		return nil, nil
	}
}
