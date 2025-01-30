package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
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

	followeeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followeeSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(toUserId).
		AddId(fromUserId).
		Build()

	followerKey := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followerSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(fromUserId).
		AddId(toUserId).
		Build()

	return repo.db.WriteTxn(func(tx *storage.WarpTxn) error {
		if err := repo.db.Set(followeeKey, data); err != nil {
			return err
		}
		return repo.db.Set(followerKey, data)
	})
}

// Unfollow removes a reader-writer relationship in both directions
func (repo *FollowRepo) Unfollow(fromUserId, toUserId string) error {
	followeeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followeeSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(toUserId).
		AddId(fromUserId).
		Build()

	followerKey := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followerSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(fromUserId).
		AddId(toUserId).
		Build()
	return repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err := repo.db.Delete(followeeKey); err != nil {
			return err
		}
		return repo.db.Delete(followerKey)
	})
}

func (repo *FollowRepo) GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error) {
	prefix := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followerSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
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
	prefix := storage.NewPrefixBuilder(FollowRepoName).
		AddRootID(followeeSubName).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
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
