package database

import (
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/security"
	"math/big"
	"time"
)

const (
	AuthRepoName    = "/AUTH"
	PassSubName     = "PASS" // TODO pass restore functionality
	DefaultOwnerKey = "OWNER"
)

var (
	ErrOwnerNotFound = errors.New("owner not found")
	ErrNilAuthRepo   = errors.New("auth repo is nil")
)

type AuthStorer interface {
	Run(username, password string) (err error)
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
}

type AuthRepo struct {
	db           AuthStorer
	owner        domain.Owner
	sessionToken string
	privateKey   crypto.PrivateKey
}

func NewAuthRepo(db AuthStorer) *AuthRepo {
	return &AuthRepo{db: db, privateKey: nil}
}

func (repo *AuthRepo) Authenticate(username, password string) (err error) {
	if repo == nil {
		return ErrNilAuthRepo
	}
	if repo.db == nil {
		return storage.ErrNotRunning
	}

	err = repo.db.Run(username, password)
	if err != nil {
		return err
	}
	repo.sessionToken, repo.privateKey, err = repo.generateSecrets(username, password)
	return err
}

func (repo *AuthRepo) generateSecrets(username, password string) (token string, pk crypto.PrivateKey, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(127))
	if err != nil {
		return "", nil, err
	}
	randChar := string(uint8(n.Uint64())) //#nosec
	feed := []byte(username + "@" + password + "@" + randChar + "@" + time.Now().String())
	token = base64.StdEncoding.EncodeToString(security.ConvertToSHA256(feed))

	seed := base64.StdEncoding.EncodeToString(
		security.ConvertToSHA256(
			[]byte(username + "@" + password + "@" + "seed"), // no random - private key must be determined
		),
	)
	privateKey, err := security.GenerateKeyFromSeed([]byte(seed))
	if err != nil {
		return "", nil, fmt.Errorf("generate private key from seed: %w", err)
	}

	return token, privateKey, nil
}

func (repo *AuthRepo) SessionToken() string {
	return repo.sessionToken
}

func (repo *AuthRepo) PrivateKey() crypto.PrivateKey {
	if repo == nil {
		return ErrNilAuthRepo
	}
	if repo.privateKey == nil {
		panic("private key is nil")
	}
	return repo.privateKey
}

func (repo *AuthRepo) GetOwner() domain.Owner {
	if repo == nil {
		panic(ErrNilAuthRepo)
	}
	if repo.owner.UserId != "" {
		return repo.owner
	}

	ownerKey := storage.NewPrefixBuilder(AuthRepoName).
		AddRootID(DefaultOwnerKey).
		Build()

	data, err := repo.db.Get(ownerKey)
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		panic(err)
	}
	if len(data) == 0 {
		return domain.Owner{}
	}

	var owner domain.Owner
	err = json.JSON.Unmarshal(data, &owner)
	if err != nil {
		panic(err)
	}
	return owner

}

func (repo *AuthRepo) SetOwner(o domain.Owner) (_ domain.Owner, err error) {
	ownerKey := storage.NewPrefixBuilder(AuthRepoName).
		AddRootID(DefaultOwnerKey).
		Build()

	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	data, err := json.JSON.Marshal(o)
	if err != nil {
		return o, err
	}

	if err = repo.db.Set(ownerKey, data); err != nil {
		return o, err
	}
	repo.owner = o
	return o, nil
}
