package websocket

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/monnand/dhkx"
)

var upgrader = websocket.Upgrader{}

// Encrypt a message using AES-GCM
func encryptMessage(secret []byte, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt a message using AES-GCM
func decryptMessage(secret []byte, ciphertext []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// Derive AES key using HKDF
func deriveKey(sharedSecret []byte) []byte {
	h := hmac.New(sha256.New, make([]byte, 32)) // Use zero-filled salt for HKDF
	h.Write(sharedSecret)
	return h.Sum(nil)[:32] // AES-256 key
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	// Generate Diffie-Hellman keys
	group, _ := dhkx.GetGroup(0) // Default group (2048 bits)
	privateKey, _ := group.GeneratePrivateKey(nil)
	publicKey := privateKey.Bytes()
	var aesKey []byte
	usedNonces := make(map[string]bool) // Track used nonces to prevent replay attacks

	// Send server's public key to the client
	if err := conn.WriteMessage(websocket.TextMessage, publicKey); err != nil {
		log.Println("Error sending public key:", err)
		return
	}

	// Receive public key from the client
	_, clientPublicKeyBytes, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error reading client's public key:", err)
		return
	}

	// Compute shared secret
	clientPublicKey := dhkx.NewPublicKey(clientPublicKeyBytes)
	sharedSecret, err := group.ComputeKey(clientPublicKey, privateKey)
	if err != nil {
		log.Println("Error computing shared secret:", err)
		return
	}
	aesKey = deriveKey(sharedSecret.Bytes())
	log.Println("Shared secret established.")

	for {
		// Read encrypted message from client
		_, encryptedMessage, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return
		}

		// Split the message into ciphertext and nonce
		parts := strings.SplitN(string(encryptedMessage), ":", 2)
		if len(parts) != 2 {
			log.Println("Invalid message format")
			return
		}
		ciphertext, _ := base64.StdEncoding.DecodeString(parts[0])
		nonce, _ := base64.StdEncoding.DecodeString(parts[1])

		// Check for replay attacks
		nonceString := string(nonce)
		if usedNonces[nonceString] {
			log.Println("Replay attack detected. Duplicate nonce:", nonceString)
			return
		}
		usedNonces[nonceString] = true

		// Decrypt the message
		plaintext, err := decryptMessage(aesKey, ciphertext, nonce)
		if err != nil {
			log.Println("Error decrypting message:", err)
			return
		}
		log.Println("Decrypted message:", string(plaintext))

		// Respond back to the client
		response := fmt.Sprintf("Echo: %s", plaintext)
		encryptedResponse, responseNonce, err := encryptMessage(aesKey, []byte(response))
		if err != nil {
			log.Println("Error encrypting response:", err)
			return
		}
		payload := fmt.Sprintf("%s:%s", base64.StdEncoding.EncodeToString(encryptedResponse), base64.StdEncoding.EncodeToString(responseNonce))
		if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
			log.Println("Error sending response:", err)
			return
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleConnection)
	log.Println("WebSocket server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
