package database

import (
	"encoding/binary"
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
	"time"
)

const (
	LikeRepoName      = "/LIKE"
	IncrSubNamespace  = "INCR"
	LikerSubNamespace = "LIKER"
)

type LikeStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	List(prefix storage.DatabaseKey, limit *uint64, cursor *string) ([]storage.ListItem, string, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	Increment(txn *storage.WarpTxn, key storage.DatabaseKey) (int64, error)
	Decrement(txn *storage.WarpTxn, key storage.DatabaseKey) (int64, error)
}

type LikeRepo struct {
	db LikeStorer
}

func NewLikeRepo(db LikeStorer) *LikeRepo {
	return &LikeRepo{db: db}
}

func (repo *LikeRepo) Like(tweetId, userId string) (likesNum int64, err error) {
	if tweetId == "" {
		return 0, errors.New("empty tweet id")
	}
	if userId == "" {
		return 0, errors.New("empty user id")
	}

	likeKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(IncrSubNamespace).
		AddRootID(tweetId).
		Build()

	likerKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(LikerSubNamespace).
		AddRootID(tweetId).
		AddReversedTimestamp(time.Now()).
		AddParentId(userId).
		Build()

	err = repo.db.WriteTxn(func(txn *storage.WarpTxn) error {
		_, err := txn.Get(likerKey.Bytes())
		if !errors.Is(err, storage.ErrKeyNotFound) {
			return nil
		}

		if err = txn.Set(likerKey.Bytes(), []byte(userId)); err != nil {
			return err
		}
		likesNum, err = repo.db.Increment(txn, likeKey)
		return err
	})
	return likesNum, err
}

func (repo *LikeRepo) Unlike(tweetId, userId string) (likesNum int64, err error) {
	if tweetId == "" {
		return 0, errors.New("empty tweet id")
	}
	if userId == "" {
		return 0, errors.New("empty user id")
	}

	likeKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(IncrSubNamespace).
		AddRootID(tweetId).
		Build()

	likerKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(LikerSubNamespace).
		AddRootID(tweetId).
		AddReversedTimestamp(time.Now()).
		AddParentId(userId).
		Build()

	err = repo.db.WriteTxn(func(txn *storage.WarpTxn) error {
		_, err := txn.Get(likeKey.Bytes())
		if errors.Is(err, storage.ErrKeyNotFound) { // already unliked
			return nil
		}
		if err = txn.Delete(likerKey.Bytes()); err != nil {
			return err
		}
		likesNum, err = repo.db.Decrement(txn, likeKey)
		return err
	})
	return likesNum, err
}

func (repo *LikeRepo) LikesCount(tweetId string) (likesNum int64, err error) {
	if tweetId == "" {
		return 0, errors.New("empty tweet id")
	}
	likeKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(IncrSubNamespace).
		AddRootID(tweetId).
		Build()

	bt, err := repo.db.Get(likeKey)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(bt)), nil
}

type likedUserIDs = []string

func (repo *LikeRepo) Likers(tweetId string, limit *uint64, cursor *string) (_ likedUserIDs, cur string, err error) {
	if tweetId == "" {
		return nil, "", errors.New("empty tweet id")
	}

	likePrefix := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(LikerSubNamespace).
		AddRootID(tweetId).
		Build()

	items, cur, err := repo.db.List(likePrefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	likers := make(likedUserIDs, 0, len(items))
	for _, item := range items {
		userId := string(item.Value)
		likers = append(likers, userId)
	}
	return likers, cur, nil
}
