package database

import (
	"encoding/binary"
	"errors"
	"github.com/filinvadim/warpnet/database/storage"
)

const (
	LikeRepoName      = "/LIKE"
	IncrSubNamespace  = "INCR"
	LikerSubNamespace = "LIKER"
)

var ErrLikesNotFound = errors.New("like not found")

type LikeStorer interface {
	Get(key storage.DatabaseKey) ([]byte, error)
	NewWriteTxn() (*storage.WarpWriteTxn, error)
	NewReadTxn() (*storage.WarpReadTxn, error)
}

type LikeRepo struct {
	db LikeStorer
}

func NewLikeRepo(db LikeStorer) *LikeRepo {
	return &LikeRepo{db: db}
}

func (repo *LikeRepo) Like(tweetId, userId string) (likesCount uint64, err error) {
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
		AddRange(storage.NoneRangeKey).
		AddParentId(userId).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	_, err = txn.Get(likerKey)
	if !errors.Is(err, storage.ErrKeyNotFound) {
		return repo.LikesCount(tweetId) // like exists
	}

	if err = txn.Set(likerKey, []byte(userId)); err != nil {
		return 0, err
	}
	likesCount, err = txn.Increment(likeKey)
	if err != nil {
		return 0, err
	}
	return likesCount, txn.Commit()
}

func (repo *LikeRepo) Unlike(userId, tweetId string) (likesCount uint64, err error) {
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
		AddRange(storage.NoneRangeKey).
		AddParentId(userId).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	_, err = txn.Get(likerKey)
	if errors.Is(err, storage.ErrKeyNotFound) { // already unliked
		return repo.LikesCount(tweetId)
	}
	if err = txn.Delete(likerKey); err != nil {
		return 0, err
	}
	likesCount, err = txn.Decrement(likeKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return likesCount, txn.Commit()
}

func (repo *LikeRepo) LikesCount(tweetId string) (likesNum uint64, err error) {
	if tweetId == "" {
		return 0, errors.New("empty tweet id")
	}
	likeKey := storage.NewPrefixBuilder(LikeRepoName).
		AddSubPrefix(IncrSubNamespace).
		AddRootID(tweetId).
		Build()

	bt, err := repo.db.Get(likeKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, ErrLikesNotFound
	}
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(bt), nil
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

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(likePrefix, limit, cursor)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return nil, "", ErrLikesNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if err = txn.Commit(); err != nil {
		return nil, "", err
	}

	likers := make(likedUserIDs, 0, len(items))
	for _, item := range items {
		userId := string(item.Value)
		likers = append(likers, userId)
	}
	return likers, cur, nil
}
