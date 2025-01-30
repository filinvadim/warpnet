package database

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	domain_gen "github.com/filinvadim/warpnet/gen/domain-gen"
	"sort"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
)

const (
	TweetsNamespace = "/TWEETS"
)

type TweetsStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	Set(key storage.DatabaseKey, value []byte) error
	List(prefix storage.DatabaseKey, limit *uint64, cursor *string) ([]storage.ListItem, string, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	Delete(key storage.DatabaseKey) error
}

type TweetRepo struct {
	db TweetsStorer
}

func NewTweetRepo(db TweetsStorer) *TweetRepo {
	return &TweetRepo{db: db}
}

// Create adds a new tweet to the database
func (repo *TweetRepo) Create(userID string, tweet domain_gen.Tweet) (domain_gen.Tweet, error) {
	if tweet == (domain_gen.Tweet{}) {
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

	data, err := json.JSON.Marshal(tweet)
	if err != nil {
		return tweet, fmt.Errorf("tweet marshal: %w", err)
	}

	err = repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Set(fixedKey, sortableKey.Bytes()); err != nil {
			return err
		}
		return repo.db.Set(sortableKey, data)
	})
	data = nil
	return tweet, err
}

// Get retrieves a tweet by its ID
func (repo *TweetRepo) Get(userID, tweetID string) (tweet domain_gen.Tweet, err error) {
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

// Delete removes a tweet by its ID
func (repo *TweetRepo) Delete(userID, tweetID string) error {
	fixedKey := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userID).
		AddRange(storage.FixedRangeKey).
		AddParentId(tweetID).
		Build()
	sortableKeyBytes, err := repo.db.Get(fixedKey)
	if err != nil {
		return err
	}
	err = repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Delete(fixedKey); err != nil {
			return err
		}
		return repo.db.Delete(storage.DatabaseKey(sortableKeyBytes))
	})
	return err
}

func (repo *TweetRepo) List(userId string, limit *uint64, cursor *string) ([]domain_gen.Tweet, string, error) {
	if userId == "" {
		return nil, "", errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(TweetsNamespace).
		AddRootID(userId).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	tweets := make([]domain_gen.Tweet, 0, len(items))
	for _, item := range items {
		var t domain_gen.Tweet
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
