package security

import (
	"crypto/sha256"
)

func ConvertToSHA256(string []byte) []byte {
	if len(string) == 0 {
		return string
	}
	hash := sha256.New()
	hash.Write(string)
	return hash.Sum(nil)
}
