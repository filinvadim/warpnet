package database

import (
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/database/storage"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
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
	ownerId      string
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
	token = base64.StdEncoding.EncodeToString(encrypting.ConvertToSHA256(feed))

	seed := base64.StdEncoding.EncodeToString(
		encrypting.ConvertToSHA256(
			[]byte(username + "@" + password + "@" + "seed"), // no random - private key must be determined
		),
	)
	privateKey, err := encrypting.GenerateKeyFromSeed([]byte(seed))
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

func (repo *AuthRepo) GetOwner() (owner domainGen.Owner, err error) {
	ownerKey := storage.NewPrefixBuilder(AuthRepoName).
		AddRootID(DefaultOwnerKey).
		Build()

	data, err := repo.db.Get(ownerKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return owner, ErrOwnerNotFound
	}
	if err != nil {
		return owner, err
	}

	err = json.JSON.Unmarshal(data, &owner)
	return owner, err

}

func (repo *AuthRepo) SetOwner(o domainGen.Owner) (_ domainGen.Owner, err error) {
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

	return o, repo.db.Set(ownerKey, data)
}
