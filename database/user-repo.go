package database

import (
	"encoding/json"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database/storage"
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
func (repo *UserRepo) Create(user api.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	if user.UserId == nil {
		id := uuid.New().String()
		user.UserId = &id
	}

	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(*user.UserId).Build()
	if err != nil {
		return err
	}
	return repo.db.Set(key, data)
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (*api.User, error) {
	key, err := storage.NewPrefixBuilder(UsersRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var user api.User
	err = json.Unmarshal(data, &user)
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
