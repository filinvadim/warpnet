/*

Warpnet - Decentralized Social Network
Copyright (C) 2025 Vadim Filin, https://github.com/filinvadim,
<github.com.mecdy@passmail.net>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Copyright 2025 Vadim Filin
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
