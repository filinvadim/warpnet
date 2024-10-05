package database

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

const (
	AuthRepoName = "AUTH"
	PassSubName  = "PASS"
	OwnerSubName = "OWNER"
)

var ErrWrongPassword = errors.New("wrong password")

type AuthRepo struct {
	db *storage.DB
}

func NewAuthRepo(db *storage.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (repo *AuthRepo) InitWithPassword(username string, password string) error {
	hashSum := crypto.ConvertToSHA256([]byte(username + "@" + password)) // aaaa + vadim

	if err := repo.db.Run(hashSum); err != nil {
		if err == badger.ErrEncryptionKeyMismatch {
			return ErrWrongPassword
		}
		return err
	}
	key, _ := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(PassSubName).Build()
	return repo.db.Set(key, hashSum)
}

func (repo *AuthRepo) IsPasswordExists() bool {
	key, _ := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(PassSubName).Build()

	p, err := repo.db.Get(key)
	if err != nil {
		return false
	}
	if p == nil {
		return false
	}
	return true
}

func (repo *AuthRepo) SetOwner(u components.User) (_ *components.User, err error) {
	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).Build()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u.CreatedAt = &now

	id := uuid.New().String()
	u.UserId = &id

	data, err := json.JSON.Marshal(u)
	if err != nil {
		return nil, err
	}
	return &u, repo.db.Set(key, data)
}

func (repo *AuthRepo) UpdateOwner(u *components.User) error {
	if u == nil {
		return errors.New("user is nil")
	}
	if u.UserId == nil {
		return errors.New("user id is missing")
	}
	if *u.UserId == "" {
		return errors.New("user id is empty")
	}

	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).Build()
	if err != nil {
		return err
	}

	bt, _ := json.JSON.Marshal(*u)
	return repo.db.Update(key, bt)
}

func (repo *AuthRepo) Owner() (*components.User, error) {
	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var u components.User
	err = json.JSON.Unmarshal(data, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
