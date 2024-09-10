package crypto

import (
	"crypto/sha256"
)

func ConvertToSHA256(password []byte) []byte {
	if len(password) == 0 {
		return password
	}
	hash := sha256.New()
	hash.Write(password)
	return hash.Sum(nil)
}
