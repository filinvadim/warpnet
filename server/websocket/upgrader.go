package websocket

import (
	"encoding/base64"
	"github.com/filinvadim/warpnet/core/encrypting"
	ws "github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

type EncryptedUpgrader struct {
	upgrader        ws.Upgrader
	conn            *ws.Conn
	readCallback    func(msg []byte) ([]byte, error)
	encrypter       *encrypting.DiffieHellmanEncrypter
	mx              *sync.Mutex
	isAuthenticated bool
	externalPubKey  []byte
	isSaltRenewed   bool
	salt            []byte
}

func NewEncryptedUpgrader() *EncryptedUpgrader {
	e, err := encrypting.NewDiffieHellmanEncrypter()
	if err != nil {
		panic(err)
	}

	return &EncryptedUpgrader{
		upgrader: ws.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				//origin := r.Header.Get("Origin")
				//addr, err := url.Parse(origin)
				//if err != nil {
				//	return false
				//}
				//if addr.Hostname() != "warp.net" {
				//	return false
				//}
				return true
			},
		},
		encrypter: e,
		mx:        new(sync.Mutex),
		salt:      []byte("warpnet"),
	}
}

func (s *EncryptedUpgrader) UpgradeConnection(w http.ResponseWriter, r *http.Request) (err error) {
	s.conn, err = s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	return s.readLoop()
}

func (s *EncryptedUpgrader) Close() error {
	return s.conn.Close()
}

func (s *EncryptedUpgrader) OnMessage(fn func(msg []byte) ([]byte, error)) {
	s.readCallback = fn
}

func (s *EncryptedUpgrader) readLoop() error {
	for {
		messageType, message, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		log.Println("got encrypted message: ", string(message))
		if messageType != ws.TextMessage {
			_ = s.SendPlain("message type must be text")
			continue
		}

		if !s.isAuthenticated {
			pubKey, err := base64.StdEncoding.DecodeString(string(message))
			if err != nil {
				_ = s.SendPlain(err.Error())
				return err
			}
			log.Println("websocket received public key message")
			s.externalPubKey = pubKey

			if err := s.encrypter.ComputeSharedSecret(s.externalPubKey, []byte("TODO")); err != nil {
				_ = s.SendPlain(err.Error())
				continue
			}
			// send ours public key
			encoded := base64.StdEncoding.EncodeToString(s.encrypter.PublicKey())
			err = s.SendPlain(encoded)
			if err != nil {
				log.Printf("Websocket error encoding public key: %s", err)
				continue
			}
			s.isAuthenticated = true
			log.Println("websocket authenticated")
			continue
		}

		if s.readCallback == nil {
			log.Println("no read callback provided")
			continue
		}
		decryptedMessage, err := s.encrypter.DecryptMessage(message)
		if err != nil {
			log.Printf("failed to decrypt message: %v", err)
			return nil
		}

		response, err := s.readCallback(decryptedMessage)
		if err != nil {
			log.Printf("failed to process decrypted message: %v", err)
		}
		if response == nil {
			continue
		}
		if err = s.SendEncrypted(response); err != nil {
			log.Printf("failed to send encrypted message: %v", err)
		}
		if err := s.renewSalt(); err != nil {
			log.Printf("failed to renew salt: %v", err)
		}
	}
}
func (s *EncryptedUpgrader) SendPlain(msg string) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	return s.conn.WriteMessage(ws.TextMessage, []byte(msg))
}
func (s *EncryptedUpgrader) SetNewSalt(salt string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.salt = []byte(salt)
	s.isSaltRenewed = false
}

func (s *EncryptedUpgrader) renewSalt() error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.isSaltRenewed {
		return nil
	}
	err := s.encrypter.ComputeSharedSecret(s.externalPubKey, s.salt)
	if err != nil {
		return err
	}
	s.isSaltRenewed = true
	log.Println("secret renewed")
	return nil
}

func (s *EncryptedUpgrader) SendEncrypted(msg []byte) error {
	encryptedMessage, err := s.encrypter.EncryptMessage(msg)
	if err != nil {
		return err
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	return s.conn.WriteMessage(ws.TextMessage, encryptedMessage)
}
