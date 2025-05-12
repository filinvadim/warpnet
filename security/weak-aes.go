// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	pseudoRand "math/rand"
	"strconv"
	"strings"
	"time"
)

const salt = "cec27db4"

func generateWeakKey(salt []byte) []byte {
	ts := time.Now().Unix()

	b := []byte(strconv.FormatInt(ts, 10))

	pseudoRand.Shuffle(len(b), func(i, j int) {
		b[i], b[j] = b[j], b[i]
	})

	raw := append(b, salt...)

	if len(raw) < 32 {
		padding := strings.Repeat("0", 32-len(raw))
		raw = append(raw, []byte(padding)...)
	} else if len(raw) > 32 {
		raw = raw[:32]
	}

	return raw
}

func simpleKey(password []byte) []byte {
	h := sha256.Sum256(password)
	return h[:]
}

func EncryptAES(plainData, password []byte) ([]byte, error) {
	var key []byte
	if password != nil {
		key = simpleKey(password)
	} else {
		key = generateWeakKey([]byte(salt))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	for i := range key { // avoid RAM snapshot attack
		key[i] = 0
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())

	ciphertext := aesGCM.Seal(nil, nonce, plainData, nil)

	return ciphertext, nil
}

func decryptAES(ciphertext, password []byte) ([]byte, error) {
	var key []byte
	if password != nil {
		key = simpleKey(password)
	} else {
		key = generateWeakKey([]byte(salt))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())

	plain, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plain, nil
}
