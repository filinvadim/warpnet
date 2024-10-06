package database

import (
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
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
func (repo *UserRepo) Create(user *domain_gen.User) (*domain_gen.User, error) {
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
	return user, repo.db.Set(key, data)
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

func (repo *UserRepo) List() ([]domain_gen.User, error) {
	key, err := storage.NewPrefixBuilder(UsersRepoName).Build()
	if err != nil {
		return nil, err
	}

	users := make([]domain_gen.User, 0, 20)
	err = repo.db.IterateKeysValues(key, func(key string, value []byte) error {
		var user domain_gen.User
		err := json.JSON.Unmarshal(value, &user)
		if err != nil {
			return err
		}
		users = append(users, user)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}
