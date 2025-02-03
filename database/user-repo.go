package database

import (
	"errors"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")

const (
	UsersRepoName    = "/USERS"
	UserSubNamespace = "USER"
)

type UserStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	Set(key storage.DatabaseKey, value []byte) error
	List(prefix storage.DatabaseKey, limit *uint64, cursor *string) ([]storage.ListItem, string, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	Delete(key storage.DatabaseKey) error
}

type UserRepo struct {
	db UserStorer
}

func NewUserRepo(db UserStorer) *UserRepo {
	return &UserRepo{db: db}
}

// Create adds a new user to the database
func (repo *UserRepo) Create(user domainGen.User) (domainGen.User, error) {
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
		AddSubPrefix(UserSubNamespace).
		AddRootID("None").
		AddRange(storage.FixedRangeKey).
		AddParentId(user.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(UsersRepoName).
		AddSubPrefix(UserSubNamespace).
		AddRootID("None").
		AddReversedTimestamp(user.CreatedAt).
		AddParentId(user.Id).
		Build()

	err = repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err = tx.Set(fixedKey.Bytes(), sortableKey.Bytes()); err != nil {
			return err
		}
		return tx.Set(sortableKey.Bytes(), data)
	})
	return user, err
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (user domainGen.User, err error) {
	if userID == "" {
		return user, ErrUserNotFound
	}
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddSubPrefix(UserSubNamespace).
		AddRootID("None").
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	sortableKeyBytes, err := repo.db.Get(fixedKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return user, ErrUserNotFound
	}
	if err != nil {
		return user, err
	}

	data, err := repo.db.Get(storage.DatabaseKey(sortableKeyBytes))
	if errors.Is(err, storage.ErrKeyNotFound) {
		return user, ErrUserNotFound
	}
	if err != nil {
		return user, err
	}

	err = json.JSON.Unmarshal(data, &user)
	if err != nil {
		return user, err
	}

	return user, nil
}

// Delete removes a user by their ID
func (repo *UserRepo) Delete(userID string) error {
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddSubPrefix(UserSubNamespace).
		AddRootID("None").
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	sortableKeyBytes, err := repo.db.Get(fixedKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	err = repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err = tx.Delete(fixedKey.Bytes()); err != nil {
			return err
		}
		return tx.Delete(sortableKeyBytes)
	})
	return err
}

func (repo *UserRepo) List(limit *uint64, cursor *string) ([]domainGen.User, string, error) {
	prefix := storage.NewPrefixBuilder(UsersRepoName).AddRootID(UserSubNamespace).Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	users := make([]domainGen.User, 0, len(items))
	for _, item := range items {
		var u domainGen.User
		err = json.JSON.Unmarshal(item.Value, &u)
		if err != nil {
			return nil, "", err
		}
		users = append(users, u)
	}

	return users, cur, nil
}
