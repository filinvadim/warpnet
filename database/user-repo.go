package database

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"time"

	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")

const (
	UsersRepoName = "USERS"
	KindUsers     = "users"
)

// UserRepo handles operations related to users
type UserRepo struct {
	db *storage.DB
}

func NewUserRepo(db *storage.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create adds a new user to the database
func (repo *UserRepo) Create(user domain_gen.User) (*domain_gen.User, error) {
	if user.UserId == nil {
		id := uuid.New().String()
		user.UserId = &id
	}
	if user.CreatedAt == nil {
		now := time.Now()
		user.CreatedAt = &now
	}
	data, err := json.JSON.Marshal(user)
	if err != nil {
		return nil, err
	}

	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddParent(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddId(*user.UserId).
		Build()

	if err = repo.db.Set(fixedKey, data); err != nil {
		return nil, err
	}

	sortableKey := storage.NewPrefixBuilder(UsersRepoName).
		AddParent(KindUsers).
		AddReversedTimestamp(*user.CreatedAt).
		AddId(*user.UserId).
		Build()

	return &user, repo.db.Set(sortableKey, data)
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (*domain_gen.User, error) {
	if userID == "" {
		return nil, ErrUserNotFound
	}
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddParent(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddId(userID).
		Build()
	data, err := repo.db.Get(fixedKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	var user domain_gen.User
	err = json.JSON.Unmarshal(data, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Delete removes a user by their ID
func (repo *UserRepo) Delete(userID string) error {
	if userID == "" {
		return ErrUserNotFound
	}
	u, err := repo.Get(userID)
	if err != nil {
		return err
	}
	sortableKey := storage.NewPrefixBuilder(UsersRepoName).
		AddParent(KindUsers).
		AddReversedTimestamp(*u.CreatedAt).
		AddId(*u.UserId).
		Build()
	if err = repo.db.Delete(sortableKey); err != nil {
		return err
	}
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddParent(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddId(userID).
		Build()
	return repo.db.Delete(fixedKey)
}

func (repo *UserRepo) List(limit *uint64, cursor *string) ([]domain_gen.User, string, error) {
	prefix := storage.NewPrefixBuilder(UsersRepoName).AddParent(KindUsers).Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	users := make([]domain_gen.User, 0, len(items))
	if err = json.JSON.Unmarshal(bytes.Join(items, []byte(",")), &users); err != nil {
		return nil, "", fmt.Errorf("%w %s", err, items)
	}

	return users, cur, nil
}
