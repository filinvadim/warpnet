package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"golang.org/x/crypto/argon2"
	"io"
	pseudoRand "math/rand"
	"strconv"
	"time"
)

type Argon2Params struct {
	Time    uint32
	Memory  uint32
	KeyLen  uint32
	SaltLen uint32
	Threads uint8
}

// DefaultArgon2Params TODO: WARNING! DO NOT DECRYPT WITH THE BELOW PARAMETERS! IT WILL LEAD TO PC OOM FAILURE
var DefaultArgon2Params = Argon2Params{
	Time:    2_000_000_000,    // iterations num
	Memory:  16 * 1024 * 1024, // 16 Gb of memory usage per tryout
	Threads: 1,                // only one thread
	KeyLen:  32,               // 256 bits for AES-256
	SaltLen: 32,
}

type Argon2 struct {
	params Argon2Params
}

func NewArgon2Cipher(params *Argon2Params) *Argon2 {
	if params == nil {
		return &Argon2{DefaultArgon2Params}
	}
	return &Argon2{*params}
}

func (a *Argon2) deriveKeyA2id(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, a.params.Time, a.params.Memory, a.params.Threads, a.params.KeyLen)
}

func (a *Argon2) generateSalt() ([]byte, error) {
	salt := make([]byte, a.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func (a *Argon2) EncryptA2id(plain, password []byte) ([]byte, error) {
	// create random salt
	salt, err := a.generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key := a.deriveKeyA2id(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, plain, nil)

	final := bytes.Join([][]byte{salt, nonce, ciphertext}, nil)

	return final, nil
}

func (a *Argon2) decryptA2id(data, password []byte) ([]byte, error) {
	if len(data) < int(a.params.SaltLen) {
		return nil, fmt.Errorf("data too short")
	}

	saltLen := a.params.SaltLen
	salt := data[:saltLen]
	rest := data[saltLen:]

	key := a.deriveKeyA2id(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(rest) < nonceSize {
		return nil, fmt.Errorf("data too short for nonce")
	}

	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	plain, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plain, nil
}

func GenerateWeakPassword() []byte {
	ts := time.Now().Unix()

	b := []byte(strconv.FormatInt(ts, 10))

	pseudoRand.Shuffle(len(b), func(i, j int) {
		b[i], b[j] = b[j], b[i]
	})

	if len(b) > 8 {
		b = b[:8]
	}
	return b
}
