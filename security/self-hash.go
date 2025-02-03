package security

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

/*
Self-hashing is a security technique where a binary computes its own cryptographic hash (e.g., SHA-256)
to verify its integrity. This ensures that the executable has not been tampered with or modified after deployment.
Key Benefits:
✔ Integrity Verification – Detects unauthorized modifications to the binary.
✔ Tamper Detection – Helps identify if the binary has been altered by malware or an attacker.
✔ P2P Security – In decentralized systems, nodes can verify their own integrity without

	a centralized trust authority.

✔ Runtime Checks – The binary can periodically rehash itself to detect modifications. TODO

Self-hashing is useful in distributed P2P networks, where nodes must ensure their integrity independently
*/
type (
	SelfHash       string
	SelfHashPrefix int
)

const (
	Bootstrap SelfHashPrefix = iota
	Member
	Business
)

// GetSelfHash calculates own executable binary file hash
func GetSelfHash(prefix SelfHashPrefix) (SelfHash, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	file, err := os.Open(exePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return SelfHash(fmt.Sprintf("%d:%x", prefix, hasher.Sum(nil))), nil
}

func (h SelfHash) Bytes() []byte {
	return []byte(h)
}

func (h SelfHash) String() string {
	return string(h)
}

func (h SelfHash) Disassemble() (SelfHashPrefix, string) {
	str := string(h)
	split := strings.Split(str, ":")
	prefix, _ := strconv.Atoi(split[0])
	return SelfHashPrefix(prefix), split[1]
}
