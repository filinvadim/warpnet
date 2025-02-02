package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"time"
)

const (
	FollowRepoName  = "/FOLLOWER"
	followeeSubName = "followee"
	followerSubName = "follower"
)

type FollowerStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	Set(key storage.DatabaseKey, value []byte) error
	List(prefix storage.DatabaseKey, limit *uint64, cursor *string) ([]storage.ListItem, string, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	Delete(key storage.DatabaseKey) error
}

// FollowRepo handles reader/writer relationships
type FollowRepo struct {
	db FollowerStorer
}

func NewFollowRepo(db FollowerStorer) *FollowRepo {
	return &FollowRepo{db: db}
}

func (repo *FollowRepo) Follow(fromUserId, toUserId string, event domain.Following) error {
	if fromUserId == "" || toUserId == "" {
		return errors.New("invalid follow params")
	}

	data, _ := json.JSON.Marshal(event)

	sortableFolloweeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(toUserId).
		AddReversedTimestamp(time.Now()).
		AddParentId(fromUserId).
		Build()

	sortableFollowerKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerSubName).
		AddRootID(fromUserId).
		AddReversedTimestamp(time.Now()).
		AddParentId(toUserId).
		Build()

	fixedFolloweeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(toUserId).
		AddRange(storage.FixedRangeKey).
		AddParentId(fromUserId).
		Build()

	fixedFollowerKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerSubName).
		AddRootID(fromUserId).
		AddRange(storage.FixedRangeKey).
		AddParentId(toUserId).
		Build()

	return repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err := repo.db.Set(sortableFollowerKey, data); err != nil {
			return err
		}
		if err := repo.db.Set(sortableFolloweeKey, data); err != nil {
			return err
		}
		if err := repo.db.Set(fixedFollowerKey, []byte(sortableFollowerKey)); err != nil {
			return err
		}
		return repo.db.Set(fixedFolloweeKey, []byte(sortableFolloweeKey))
	})
}

// Unfollow removes a reader-writer relationship in both directions
func (repo *FollowRepo) Unfollow(fromUserId, toUserId string) error {
	fixedFolloweeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(toUserId).
		AddRange(storage.FixedRangeKey).
		AddParentId(fromUserId).
		Build()

	fixedFollowerKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerSubName).
		AddRootID(fromUserId).
		AddRange(storage.FixedRangeKey).
		AddParentId(toUserId).
		Build()

	sortableFollowerKey, err := repo.db.Get(fixedFollowerKey)
	if err != nil {
		return err
	}
	sortableFolloweeKey, err := repo.db.Get(fixedFolloweeKey)
	if err != nil {
		return err
	}
	return repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err := repo.db.Delete(fixedFolloweeKey); err != nil {
			return err
		}
		if err := repo.db.Delete(fixedFollowerKey); err != nil {
			return err
		}
		if err := repo.db.Delete(storage.DatabaseKey(sortableFollowerKey)); err != nil {
			return err
		}
		return repo.db.Delete(storage.DatabaseKey(sortableFolloweeKey))
	})
}

func (repo *FollowRepo) GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error) {
	followeePrefix := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(userId).
		Build()

	items, cur, err := repo.db.List(followeePrefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	followings := make([]domain.Following, 0, len(items))
	for _, item := range items {
		var f domain.Following
		err = json.JSON.Unmarshal(item.Value, &f)
		if err != nil {
			return nil, "", err
		}
		followings = append(followings, f)
	}

	return followings, cur, nil
}

// GetFollowees : followee - one who is followed (has his/her posts monitored by another user)
func (repo *FollowRepo) GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error) {
	followerPrefix := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerSubName).
		AddRootID(userId).
		Build()

	items, cur, err := repo.db.List(followerPrefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	followings := make([]domain.Following, 0, len(items))
	for _, item := range items {
		var f domain.Following
		err = json.JSON.Unmarshal(item.Value, &f)
		if err != nil {
			return nil, "", err
		}
		followings = append(followings, f)
	}

	return followings, cur, nil
}
