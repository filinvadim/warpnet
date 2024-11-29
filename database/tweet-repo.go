package database

import (
	"errors"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sort"
	"time"

	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

const TweetsRepoName = "TWEETS"

// TweetRepo handles operations related to tweets
type TweetRepo struct {
	db *storage.DB
}

func NewTweetRepo(db *storage.DB) *TweetRepo {
	return &TweetRepo{db: db}
}

// Create adds a new tweet to the database
func (repo *TweetRepo) Create(userID string, tweet *domain_gen.Tweet) (*domain_gen.Tweet, error) {
	if tweet == nil {
		return nil, errors.New("nil tweet")
	}
	if tweet.TweetId == nil {
		id := uuid.New().String()
		tweet.TweetId = &id
	}
	if tweet.CreatedAt == nil {
		now := time.Now()
		tweet.CreatedAt = &now
	}

	data, err := json.JSON.Marshal(*tweet)
	if err != nil {
		return nil, fmt.Errorf("tweet marshal: %w", err)
	}

	key := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(*tweet.TweetId).Build()

	return tweet, repo.db.Set(key, data)
}

// Get retrieves a tweet by its ID
func (repo *TweetRepo) Get(userID, tweetID string) (*domain_gen.Tweet, error) {
	key := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(tweetID).Build()
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var tweet domain_gen.Tweet
	err = json.JSON.Unmarshal(data, &tweet)
	if err != nil {
		return nil, err
	}
	return &tweet, nil
}

// Delete removes a tweet by its ID
func (repo *TweetRepo) Delete(userID, tweetID string) error {
	key := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(tweetID).Build()
	return repo.db.Delete(key)
}

func (repo *TweetRepo) List(userId string, limit *uint64, cursor *string) ([]domain_gen.Tweet, string, error) {
	if userId == "" {
		return nil, "", errors.New("user ID cannot be blank")
	}
	if limit == nil {
		limit = new(uint64)
		*limit = 20
	}

	prefix := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userId).Build()

	if cursor != nil && *cursor != "" {
		prefix = storage.DatabaseKey(*cursor)
	}

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	tweets := make([]domain_gen.Tweet, 0, *limit)
	if err = json.JSON.Unmarshal(items, &tweets); err != nil {
		return nil, "", err
	}

	sort.SliceStable(tweets, func(i, j int) bool {
		return tweets[i].CreatedAt.After(*tweets[j].CreatedAt)
	})

	return tweets, cur, nil
}
