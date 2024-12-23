package node_crypto

import (
	go_crypto "crypto"
	"crypto/sha256"
	"errors"
	"github.com/libp2p/go-libp2p/core/crypto"
)

func ConvertToSHA256(password []byte) []byte {
	if len(password) == 0 {
		return password
	}
	//hashAlgo := go_crypto.SHA256

	hash := sha256.New()
	hash.Write(password)
	return hash.Sum(nil)
}

type PrivateKey crypto.PrivKey

func GenerateKeyFromSeed(seed []byte) (go_crypto.PrivateKey, error) {
	if len(seed) == 0 {
		return nil, errors.New("seed is empty")
	}
	hashAlgo := go_crypto.SHA256
	seed = append(seed, uint8(hashAlgo))
	hash := sha256.Sum256(seed)
	return crypto.UnmarshalPrivateKey(hash[:])
}
