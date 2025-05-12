// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package security

import (
	"bytes"
	go_crypto "crypto"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/crypto/pb"
)

type PrivateKey crypto.PrivKey

func GenerateKeyFromSeed(seed []byte) (go_crypto.PrivateKey, error) {
	if len(seed) == 0 {
		return nil, errors.New("seed is empty")
	}
	hashAlgo := go_crypto.SHA256
	keyType := pb.KeyType_Ed25519
	seed = append(seed, uint8(hashAlgo))
	seed = append(seed, uint8(keyType))
	hash := sha256.Sum256(seed)
	privKey, _, err := crypto.GenerateEd25519Key(bytes.NewReader(hash[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return privKey, nil
}
