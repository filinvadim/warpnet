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

package security

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestAESEncryptDecrypt_Success(t *testing.T) {
	ts := time.Now().UnixNano()

	b := []byte(strconv.FormatInt(ts, 10))

	fmt.Println(len(b), "??????????")

	password := []byte("SuperSecretPassword123!")
	in := []byte("Hello, this is a secret message.")

	fmt.Println("plaintext:", string(in))

	cipherData, err := EncryptAES(in, password)
	assert.NoError(t, err)

	out, err := decryptAES(cipherData, password)
	assert.NoError(t, err)

	fmt.Println("decrypted:", string(out))

	assert.Equal(t, in, out)
}
