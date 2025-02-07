package database

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"sort"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
)

const (
	TweetsNamespace       = "/TWEETS"
	tweetsCountSubspace   = "TWEETSCOUNT"
	reTweetsCountSubspace = "RETWEETSCOUNT"
)

type TweetsStorer interface {
	NewWriteTxn() (*storage.WarpWriteTxn, error)
	NewReadTxn() (*storage.WarpReadTxn, error)
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
	Delete(key storage.DatabaseKey) error
}

type TweetRepo struct {
	db        TweetsStorer
	tweetsNum int64
}

func NewTweetRepo(db TweetsStorer) *TweetRepo {
	return &TweetRepo{db: db}
}

// Create adds a new tweet to the database
func (repo *TweetRepo) Create(userID string, tweet domain.Tweet) (domain.Tweet, error) {
	if tweet == (domain.Tweet{}) {
		return tweet, errors.New("nil tweet")
	}
	if tweet.Id == "" {
		tweet.Id = uuid.New().String()
	}
	if tweet.CreatedAt.IsZero() {
		tweet.CreatedAt = time.Now()
	}
	tweet.RootId = tweet.Id

	fixedKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userID).
		AddRange(storage.FixedRangeKey).
		AddParentId(tweet.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userID).
		AddReversedTimestamp(tweet.CreatedAt).
		AddParentId(tweet.Id).
		Build()

	countKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddSubPrefix(tweetsCountSubspace).
		AddRootID(userID).
		Build()

	var (
		retweetCountKey storage.DatabaseKey
		isRetweeted     bool
	)
	if tweet.RetweetedBy != nil {
		retweetCountKey = storage.NewPrefixBuilder(TweetsNamespace).
			AddSubPrefix(reTweetsCountSubspace).
			AddRootID(tweet.Id).
			Build()

		_, err := repo.db.Get(retweetCountKey)
		if !errors.Is(err, storage.ErrKeyNotFound) {
			isRetweeted = true // already retweeted exists
		}
	}

	data, err := json.JSON.Marshal(tweet)
	if err != nil {
		return tweet, fmt.Errorf("tweet marshal: %w", err)
	}

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return tweet, err
	}
	defer txn.Rollback()

	if err = txn.Set(fixedKey, sortableKey.Bytes()); err != nil {
		return tweet, err
	}
	if err = txn.Set(sortableKey, data); err != nil {
		return tweet, err
	}
	if _, err := txn.Increment(countKey); err != nil {
		return tweet, err
	}
	if tweet.RetweetedBy != nil && !isRetweeted {
		_, err = txn.Increment(retweetCountKey)
		if err != nil {
			return tweet, err
		}
	}
	return tweet, txn.Commit()
}

// Get retrieves a tweet by its ID
func (repo *TweetRepo) Get(userID, tweetID string) (tweet domain.Tweet, err error) {
	fixedKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userID).
		AddRange(storage.FixedRangeKey).
		AddParentId(tweetID).
		Build()
	sortableKeyBytes, err := repo.db.Get(fixedKey)
	if err != nil {
		return tweet, err
	}

	data, err := repo.db.Get(storage.DatabaseKey(sortableKeyBytes))
	if err != nil {
		return tweet, err
	}

	err = json.JSON.Unmarshal(data, &tweet)
	if err != nil {
		return tweet, err
	}
	data = nil
	return tweet, nil
}

func (repo *TweetRepo) ReTweetsCount(tweetId string) (uint64, error) {
	if tweetId == "" {
		return 0, errors.New("retweets count: empty tweet id")
	}
	retweetCountKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddSubPrefix(reTweetsCountSubspace).
		AddRootID(tweetId).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	bt, err := txn.Get(retweetCountKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := binary.BigEndian.Uint64(bt)
	return count, txn.Commit()
}

func (repo *TweetRepo) TweetsCount(userID string) (uint64, error) {
	if userID == "" {
		return 0, errors.New("tweet count: empty userID")
	}
	countKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddSubPrefix(tweetsCountSubspace).
		AddRootID(userID).
		Build()
	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()
	bt, err := txn.Get(countKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := binary.BigEndian.Uint64(bt)
	return count, txn.Commit()
}

// Delete removes a tweet by its ID
func (repo *TweetRepo) Delete(userID, tweetID string) error {
	countKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddSubPrefix(tweetsCountSubspace).
		AddRootID(userID).
		Build()

	fixedKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userID).
		AddRange(storage.FixedRangeKey).
		AddParentId(tweetID).
		Build()
	sortableKeyBytes, err := repo.db.Get(fixedKey)
	if err != nil {
		return err
	}

	bt, err := repo.db.Get(storage.DatabaseKey(sortableKeyBytes))
	if err != nil {
		return err
	}

	var tweet domain.Tweet
	err = json.JSON.Unmarshal(bt, &tweet)
	if err != nil {
		return err
	}

	var retweetCountKey storage.DatabaseKey
	if tweet.RetweetedBy != nil {
		retweetCountKey = storage.NewPrefixBuilder(TweetsNamespace).
			AddSubPrefix(reTweetsCountSubspace).
			AddRootID(tweet.Id).
			Build()
	}

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if err := txn.Delete(fixedKey); err != nil {
		return err
	}
	if err := txn.Delete(storage.DatabaseKey(sortableKeyBytes)); err != nil {
		return err
	}
	if _, err := txn.Decrement(countKey); err != nil {
		return err
	}
	if tweet.RetweetedBy != nil {
		_, err = txn.Decrement(retweetCountKey)
		if err != nil {
			return err
		}
	}
	return txn.Commit()
}

func (repo *TweetRepo) List(userId string, limit *uint64, cursor *string) ([]domain.Tweet, string, error) {
	if userId == "" {
		return nil, "", errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userId).
		Build()

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	if err := txn.Commit(); err != nil {
		return nil, "", err
	}

	tweets := make([]domain.Tweet, 0, len(items))
	for _, item := range items {
		var t domain.Tweet
		err = json.JSON.Unmarshal(item.Value, &t)
		if err != nil {
			return nil, "", err
		}
		tweets = append(tweets, t)
	}
	sort.SliceStable(tweets, func(i, j int) bool {
		return tweets[i].CreatedAt.After(tweets[j].CreatedAt)
	})
	return tweets, cur, nil
}
