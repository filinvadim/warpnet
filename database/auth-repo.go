package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
	"time"
)

const (
	AuthRepoName = "AUTH"
	PassSubName  = "PASS"
	OwnerSubName = "OWNER"
	CASubName    = "CA"
)

var ErrWrongPassword = errors.New("wrong password")

type AuthRepo struct {
	db *storage.DB
}

func NewAuthRepo(db *storage.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (repo *AuthRepo) InitWithPassword(h []byte) error {
	if err := repo.db.Run(h); err != nil {
		if err == badger.ErrEncryptionKeyMismatch {
			return ErrWrongPassword
		}
		return err
	}
	key, _ := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(PassSubName).Build()
	return repo.db.Set(key, h)
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

func (repo *AuthRepo) SetCA(CA []byte) error {
	key, _ := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(CASubName).Build()
	return repo.db.Set(key, CA)
}

func (repo *AuthRepo) IsCAExists() bool {
	key, _ := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(CASubName).Build()

	p, err := repo.db.Get(key)
	if err != nil {
		return false
	}
	if p == nil {
		return false
	}
	return true
}

func (repo *AuthRepo) SetOwner(u *server.User) (_ *server.User, err error) {
	if u == nil {
		return nil, errors.New("user is nil")
	}
	if u.UserId == nil {
		id := uuid.New().String()
		u.UserId = &id
	}
	if u.CreatedAt == nil {
		now := time.Now()
		u.CreatedAt = &now
	}
	//if u.ReferredBy == nil {
	//	return errors.New("")
	//}
	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).AddUserId(*u.UserId).Build()
	if err != nil {
		return nil, err
	}
	data, err := json.JSON.Marshal(u)
	if err != nil {
		return nil, err
	}
	_, err = repo.GetOwner(*u.UserId)
	if err == nil {
		return nil, errors.New("user already exists")
	}
	if err != badger.ErrKeyNotFound {
		return nil, err
	}

	return u, repo.db.Set(key, data)
}

func (repo *AuthRepo) UpdateOwner(u *server.User) error {
	if u == nil {
		return errors.New("user is nil")
	}
	if u.UserId == nil {
		return errors.New("user id is missing")
	}
	if *u.UserId == "" {
		return errors.New("user id is empty")
	}

	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).AddUserId(*u.UserId).Build()
	if err != nil {
		return err
	}

	bt, err := json.JSON.Marshal(*u)
	return repo.db.Update(key, bt)
}

func (repo *AuthRepo) GetOwner(userId string) (*server.User, error) {
	key, err := storage.NewPrefixBuilder(AuthRepoName).AddPrefix(OwnerSubName).AddUserId(userId).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var u server.User
	err = json.JSON.Unmarshal(data, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
