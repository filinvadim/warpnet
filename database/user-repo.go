package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	domain_gen "github.com/filinvadim/warpnet/domain-gen"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
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
func (repo *UserRepo) Create(user domain_gen.User) (domain_gen.User, error) {
	if user.Id == "" {
		user.Id = uuid.New().String()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	data, err := json.JSON.Marshal(user)
	if err != nil {
		return user, err
	}

	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddParentId(user.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(KindUsers).
		AddReversedTimestamp(user.CreatedAt).
		AddParentId(user.Id).
		Build()

	err = repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Set(fixedKey, data); err != nil {
			return err
		}
		return repo.db.Set(sortableKey, data)
	})
	return user, err
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (user domain_gen.User, err error) {
	if userID == "" {
		return user, ErrUserNotFound
	}
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	data, err := repo.db.Get(fixedKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return user, ErrUserNotFound
	}
	if err != nil {
		return user, err
	}

	err = json.JSON.Unmarshal(data, &user)
	if err != nil {
		return user, err
	}

	if user.Id == "" {
		user.Id = userID
	}
	return user, nil
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
		AddRootID(KindUsers).
		AddReversedTimestamp(u.CreatedAt).
		AddParentId(u.Id).
		Build()
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(KindUsers).
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	err = repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Delete(fixedKey); err != nil {
			return err
		}
		return repo.db.Delete(sortableKey)
	})
	return err
}

func (repo *UserRepo) List(limit *uint64, cursor *string) ([]domain_gen.User, string, error) {
	prefix := storage.NewPrefixBuilder(UsersRepoName).AddRootID(KindUsers).Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	users := make([]domain_gen.User, 0, len(items))
	for _, item := range items {
		var u domain_gen.User
		err = json.JSON.Unmarshal(item.Value, &u)
		if err != nil {
			return nil, "", err
		}
		users = append(users, u)
	}

	return users, cur, nil
}
