package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"

	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
)

const (
	AuthRepoName = "AUTH"
	PassSubName  = "PASS" // TODO pass restore functionality
	OwnerSubName = "OWNER"
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrOwnerNotFound = errors.New("owner not found")
)

type AuthRepo struct {
	db      *storage.DB
	ownerId string
}

func NewAuthRepo(db *storage.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (repo *AuthRepo) Authenticate(username, password string) (token string, err error) {
	token, err = repo.db.Run(username, password)
	if err != nil {
		return "", ErrWrongPassword
	}
	return token, nil
}

func (repo *AuthRepo) NewOwner() (userId string, err error) {
	fixedKey := storage.NewPrefixBuilder(AuthRepoName).
		AddKind(OwnerSubName).
		Build()

	id := uuid.New().String()
	if err := repo.db.Set(fixedKey, []byte(id)); err != nil {
		return "", err
	}

	repo.ownerId = id
	return id, nil
}

func (repo *AuthRepo) Owner() (userId string, err error) {
	if repo.ownerId != "" {
		return repo.ownerId, nil
	}
	fixedKey := storage.NewPrefixBuilder(AuthRepoName).
		AddKind(OwnerSubName).
		Build()
	data, err := repo.db.Get(fixedKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return "", ErrOwnerNotFound
	}
	if err != nil {
		return "", err
	}

	return string(data), nil
}
