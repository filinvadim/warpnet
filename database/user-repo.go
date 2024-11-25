package database

import (
	"errors"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sort"
	"time"

	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

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
	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
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
	if *limit == 0 {
		limit = new(uint64)
		*limit = 20
	}
	prefix, err := storage.NewPrefixBuilder(UsersRepoName).Build()
	if err != nil {
		return nil, "", err
	}

	var lastKey string
	if cursor != nil && *cursor != "" {
		prefix = *cursor
	}

	users := make([]domain_gen.User, 0, *limit)
	err = repo.db.IterateKeysValues(prefix, func(key string, value []byte) error {
		if len(users) >= int(*limit) {
			lastKey = key
			return storage.ErrStopIteration
		}
		if !IsValidForPrefix(key, prefix) {
			return nil
		}

		var u domain_gen.User
		err := json.JSON.Unmarshal(value, &u)
		if err != nil {
			return err
		}
		users = append(users, u)
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if errors.Is(err, storage.ErrStopIteration) || err == nil {
		if len(users) < int(*limit) {
			lastKey = ""
		}
		return users, lastKey, nil
	}

	sort.SliceStable(users, func(i, j int) bool {
		return users[i].CreatedAt.After(*users[j].CreatedAt)
	})

	return users, lastKey, nil
}
