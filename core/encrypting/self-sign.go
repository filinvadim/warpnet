package encrypting

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"
)

func VerifySelfSignature(signature, publicKeyData []byte) bool {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to get executable path: %v", err)
	}

	file, err := os.Open(exePath)
	if err != nil {
		log.Fatalf("failed to open binary file: %v", err)
	}
	defer file.Close()

	binaryData, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("failed to read binary file: %v", err)
	}
	hash := sha256.Sum256(binaryData)
	publicKey, err := loadEd25519PublicKey(publicKeyData)
	if err != nil {
		log.Fatalf("failed to load public key: %v", err)
	}
	return ed25519.Verify(publicKey, hash[:], signature)
}

func loadEd25519PublicKey(keyData []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ed25519 public key: %w", err)
	}

	ed25519Key, ok := pubKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}

	return ed25519Key, nil
}
