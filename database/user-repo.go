package database

import (
	"errors"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")

const (
	UsersRepoName = "USERS"
	defaultRootID = "users"

	DefaultOwnerUserID = "OWNER"
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
		AddRootID(defaultRootID).
		AddRange(storage.FixedRangeKey).
		AddParentId(user.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(defaultRootID).
		AddReversedTimestamp(user.CreatedAt).
		AddParentId(user.Id).
		Build()

	err = repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err = repo.db.Set(fixedKey, data); err != nil {
			return err
		}
		return repo.db.Set(sortableKey, data)
	})
	return user, err
}

// Get retrieves a user by their ID
func (repo *UserRepo) Get(userID string) (user domainGen.User, err error) {
	if userID == "" {
		return user, ErrUserNotFound
	}
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(defaultRootID).
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	data, err := repo.db.Get(fixedKey)
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
		AddRootID(defaultRootID).
		AddReversedTimestamp(u.CreatedAt).
		AddParentId(u.Id).
		Build()
	fixedKey := storage.NewPrefixBuilder(UsersRepoName).
		AddRootID(defaultRootID).
		AddRange(storage.FixedRangeKey).
		AddParentId(userID).
		Build()
	err = repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err = repo.db.Delete(fixedKey); err != nil {
			return err
		}
		return repo.db.Delete(sortableKey)
	})
	return err
}

func (repo *UserRepo) List(limit *uint64, cursor *string) ([]domainGen.User, string, error) {
	prefix := storage.NewPrefixBuilder(UsersRepoName).AddRootID(defaultRootID).Build()

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

func (repo *UserRepo) Owner() (domainGen.Owner, error) {
	owner, err := repo.Get(DefaultOwnerUserID)
	if errors.Is(err, ErrUserNotFound) {
		return domainGen.Owner{}, ErrUserNotFound
	}
	return domainGen.Owner{
		CreatedAt:   owner.CreatedAt,
		Description: owner.Description,
		Id:          owner.Id,
		NodeId:      owner.NodeId,
		Username:    owner.Username,
	}, err
}

func (repo *UserRepo) CreateOwner(o domainGen.Owner) (err error) {
	_, err = repo.Create(domainGen.User{
		Birthdate:   nil,
		CreatedAt:   o.CreatedAt,
		Description: o.Description,
		Id:          DefaultOwnerUserID,
		NodeId:      o.NodeId,
		Username:    o.Username,
	})
	return err
}
