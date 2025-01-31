package encrypting

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type (
	SelfHash       string
	SelfHashPrefix int
)

const (
	Bootstrap = iota
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
