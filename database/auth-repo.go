package database

import (
	"crypto"
	"encoding/base64"
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
	node_crypto "github.com/filinvadim/warpnet/node-crypto"
	"math/rand/v2"
	"time"
)

const (
	AuthRepoName = "AUTH"
	PassSubName  = "PASS" // TODO pass restore functionality
	OwnerSubName = "owner"
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrOwnerNotFound = errors.New("owner not found")
)

type AuthRepo struct {
	db             *storage.DB
	ownerId        string
	sessionToken   string
	privateKeyChan chan crypto.PrivateKey
}

func NewAuthRepo(db *storage.DB) *AuthRepo {
	return &AuthRepo{db: db, privateKeyChan: make(chan crypto.PrivateKey, 1)}
}

func (repo *AuthRepo) Authenticate(username, password string) (token string, err error) {
	if repo.db == nil {
		return "", storage.ErrNotRunning
	}
	err = repo.db.Run(username, password)
	if err != nil {
		return "", ErrWrongPassword
	}

	randChar := string(uint8(rand.Uint()))
	feed := []byte(username + "@" + password + "@" + randChar + "@" + time.Now().String())
	repo.sessionToken = base64.StdEncoding.EncodeToString(node_crypto.ConvertToSHA256(feed))

	seed := base64.StdEncoding.EncodeToString(
		node_crypto.ConvertToSHA256(
			[]byte(username + "@" + password + "@" + "seed"),
		),
	)
	privateKey, err := node_crypto.GenerateKeyFromSeed([]byte(seed))
	if err != nil {
		return "", err
	}
	repo.privateKeyChan <- privateKey
	return token, nil
}

func (repo *AuthRepo) SessionToken() string {
	return repo.sessionToken
}

func (repo *AuthRepo) ReadyChan() <-chan crypto.PrivateKey {
	return repo.privateKeyChan
}
