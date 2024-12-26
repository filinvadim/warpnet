package core

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"math/big"
	"strconv"
	"time"
)

func proofOfWork(data []byte, difficulty int) string {
	var (
		nonce      int64
		targetHash string
		timestamp  = time.Now().UnixMilli()
	)

	// Perform the proof of work
	target := big.NewInt(1)
	target.Lsh(target, uint(256-difficulty))
	for nonce < math.MaxInt64 {
		hash := calculateHash(data, nonce, timestamp)

		hashInt := new(big.Int)
		hashInt.SetString(hash, 16)

		if hashInt.Cmp(target) == -1 {
			targetHash = hash
			break
		} else {
			nonce++
		}
	}

	return targetHash
}

func calculateHash(data []byte, nonce, timestamp int64) string {
	record := strconv.FormatInt(timestamp, 10) + string(data) + strconv.FormatInt(nonce, 10)

	hash := sha256.Sum256([]byte(record))
	return hex.EncodeToString(hash[:])
}
