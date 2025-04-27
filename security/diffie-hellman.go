package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/monnand/dhkx"
	"golang.org/x/crypto/hkdf"
	"io"
	"strings"
	"sync"
)

type DiffieHellmanEncrypter struct {
	usedNonces *sync.Map
	aesKey     []byte
	publicKey  []byte
	privateKey *dhkx.DHKey
	group      *dhkx.DHGroup
}

func NewDiffieHellmanEncrypter() (*DiffieHellmanEncrypter, error) {
	group, err := dhkx.GetGroup(0) // Default group (2048 bits)
	if err != nil {
		return nil, err
	}
	privateKey, err := group.GeneratePrivateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	publicKey := privateKey.Bytes()
	return &DiffieHellmanEncrypter{
		usedNonces: new(sync.Map),
		aesKey:     nil,
		publicKey:  publicKey,
		privateKey: privateKey,
		group:      group,
	}, nil
}

// Encrypt a message using AES-GCM
func (e *DiffieHellmanEncrypter) EncryptMessage(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.aesKey)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil) //#nosec
	payload := fmt.Sprintf(
		"%s:%s",
		base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(nonce))

	return []byte(payload), nil
}

func (e *DiffieHellmanEncrypter) PublicKey() []byte {
	return e.publicKey
}

// DecryptMessage a message using AES-GCM
func (e *DiffieHellmanEncrypter) DecryptMessage(encryptedMessage []byte) ([]byte, error) {
	parts := strings.SplitN(string(encryptedMessage), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid client message format: %s", encryptedMessage)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decoding text: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding nonce: %w", err)
	}

	// Check for replay attacks
	nonceString := string(nonce)
	if _, ok := e.usedNonces.Load(nonceString); ok {
		return nil, errors.New("replay attack detected")
	}
	e.usedNonces.Store(nonceString, struct{}{})
	e.usedNonces.Range(func(k, v interface{}) bool {
		e.usedNonces.Delete(k.(string))
		return false
	})

	block, err := aes.NewCipher(e.aesKey)
	if err != nil {
		return nil, fmt.Errorf("new aes block: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new aesgcm cipher: %w", err)
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("aesgcm decrypt: %w %x", err, ciphertext)
	}

	return plaintext, nil
}

func (e *DiffieHellmanEncrypter) ComputeSharedSecret(clientPublicKeyBytes, salt []byte) error {
	e.usedNonces.Clear()

	clientPublicKey := dhkx.NewPublicKey(clientPublicKeyBytes)
	sharedSecret, err := e.group.ComputeKey(clientPublicKey, e.privateKey)
	if err != nil {
		return fmt.Errorf("computing shared secret: %v", err)
	}
	e.aesKey, err = deriveKeyDH(sharedSecret.Bytes(), salt)
	return err
}

// Derive AES key using HKDF
func deriveKeyDH(sharedSecret, salt []byte) ([]byte, error) {
	h := hkdf.New(sha256.New, sharedSecret, salt, []byte(""))
	key := make([]byte, 32) // AES-256
	_, err := io.ReadFull(h, key)
	return key, err
}
