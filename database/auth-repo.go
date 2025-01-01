package database

import (
	"crypto"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/database/storage"
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
	ErrNilAuthRepo   = errors.New("auth repo is nil")
)

type AuthStorer interface {
	Run(username, password string) (err error)
}

type AuthRepo struct {
	db           AuthStorer
	ownerId      string
	sessionToken string
	privateKey   crypto.PrivateKey
}

func NewAuthRepo(db AuthStorer) *AuthRepo {
	return &AuthRepo{db: db, privateKey: nil}
}

func (repo *AuthRepo) Authenticate(username, password string) (token string, err error) {
	if repo == nil {
		return "", ErrNilAuthRepo
	}
	if repo.db == nil {
		return "", storage.ErrNotRunning
	}

	randChar := string(uint8(rand.Int()))
	feed := []byte(username + "@" + password + "@" + randChar + "@" + time.Now().String())
	repo.sessionToken = base64.StdEncoding.EncodeToString(encrypting.ConvertToSHA256(feed))

	seed := base64.StdEncoding.EncodeToString(
		encrypting.ConvertToSHA256(
			[]byte(username + "@" + password + "@" + "seed"),
		),
	)
	privateKey, err := encrypting.GenerateKeyFromSeed([]byte(seed))
	if err != nil {
		return "", fmt.Errorf("generate key from seed: %w", err)
	}

	err = repo.db.Run(username, password)
	if err != nil {
		return "", err
	}
	repo.privateKey = privateKey
	return token, nil
}

func (repo *AuthRepo) SessionToken() string {
	return repo.sessionToken
}

func (repo *AuthRepo) PrivateKey() crypto.PrivateKey {
	if repo == nil {
		return ErrNilAuthRepo
	}
	return repo.privateKey
}
