package database

import (
	"encoding/binary"
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/json"
	"time"
)

const (
	FollowRepoName       = "/FOLLOWER"
	followeeSubName      = "FOLLOWEE"
	followerSubName      = "FOLLOWER"
	followeeCountSubName = "FOLLOWEECOUNT"
	followerCountSubName = "FOLLOWERCOUNT"
)

type FollowerStorer interface {
	NewWriteTxn() (storage.WarpTxWriter, error)
	NewReadTxn() (storage.WarpTxReader, error)
	Set(key storage.DatabaseKey, value []byte) error
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

	followeesCountKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeCountSubName).
		AddRootID(toUserId).
		Build()

	followersCountKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerCountSubName).
		AddRootID(fromUserId).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if err := txn.Set(sortableFollowerKey, data); err != nil {
		return err
	}
	if err := txn.Set(sortableFolloweeKey, data); err != nil {
		return err
	}
	if err := txn.Set(fixedFollowerKey, []byte(sortableFollowerKey)); err != nil {
		return err
	}
	if err := txn.Set(fixedFolloweeKey, []byte(sortableFolloweeKey)); err != nil {
		return err
	}
	if _, err := txn.Increment(followersCountKey); err != nil {
		return err
	}
	if _, err := txn.Increment(followeesCountKey); err != nil {
		return err
	}
	return txn.Commit()
}

func (repo *FollowRepo) Unfollow(fromUserId, toUserId string) error {
	fixedFolloweeKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(toUserId).
		AddRange(storage.FixedRangeKey).
		AddParentId(fromUserId).
		Build()

	followeesCountKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeCountSubName).
		AddRootID(toUserId).
		Build()

	sortableFolloweeKey, err := repo.db.Get(fixedFolloweeKey)
	if err != nil {
		return err
	}
	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if err := txn.Delete(fixedFolloweeKey); err != nil {
		return err
	}
	if err := txn.Delete(storage.DatabaseKey(sortableFolloweeKey)); err != nil {
		return err
	}
	if _, err := txn.Decrement(followeesCountKey); err != nil {
		return err
	}
	return txn.Commit()
}

func (repo *FollowRepo) GetFollowersCount(userId string) (uint64, error) {
	if userId == "" {
		return 0, errors.New("followers count: empty userID")
	}
	followersCountKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followerCountSubName).
		AddRootID(userId).
		Build()
	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()
	bt, err := txn.Get(followersCountKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := binary.BigEndian.Uint64(bt)
	return count, txn.Commit()
}

func (repo *FollowRepo) GetFolloweesCount(userId string) (uint64, error) {
	if userId == "" {
		return 0, errors.New("followers count: empty userID")
	}
	followeesCountKey := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeCountSubName).
		AddRootID(userId).
		Build()
	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()
	bt, err := txn.Get(followeesCountKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := binary.BigEndian.Uint64(bt)
	return count, txn.Commit()
}

func (repo *FollowRepo) GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error) {
	followeePrefix := storage.NewPrefixBuilder(FollowRepoName).
		AddSubPrefix(followeeSubName).
		AddRootID(userId).
		Build()

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(followeePrefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	if err := txn.Commit(); err != nil {
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

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(followerPrefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	if err := txn.Commit(); err != nil {
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
