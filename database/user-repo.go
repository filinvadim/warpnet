package database

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sort"
	"time"

	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")

const UsersRepoName = "USERS"

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

	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(*user.UserId).Build()
	if err != nil {
		return nil, err
	}

	return &user, repo.db.Set(key, data)
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (*domain_gen.User, error) {
	if userID == "" {
		return nil, ErrUserNotFound
	}
	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
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
	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(userID).Build()
	if err != nil {
		return err
	}
	return repo.db.Delete(key)
}

func (repo *UserRepo) List(limit *uint64, cursor *string) ([]domain_gen.User, string, error) {
	if limit == nil {
		limit = new(uint64)
		*limit = 20
	}

	prefix, err := storage.NewPrefixBuilder(UsersRepoName).Build()
	if err != nil {
		return nil, "", err
	}

	if cursor != nil && *cursor != "" {
		prefix = storage.DatabaseKey(*cursor)
	}

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	users := make([]domain_gen.User, 0, *limit)
	if err = json.JSON.Unmarshal(items, &users); err != nil {
		return nil, "", fmt.Errorf("%w %s", err, items)
	}

	sort.SliceStable(users, func(i, j int) bool {
		return users[i].CreatedAt.After(*users[j].CreatedAt)
	})

	return users, cur, nil
}
