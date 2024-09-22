package database

import (
	"fmt"
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
	"time"
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
func (repo *TweetRepo) Create(userID string, tweet server.Tweet) (*server.Tweet, error) {

	if tweet.TweetId == nil {
		id := uuid.New().String()
		tweet.TweetId = &id
	}
	if tweet.CreatedAt == nil {
		now := time.Now()
		tweet.CreatedAt = &now
	}
	if tweet.Sequence == nil {
		seq, err := repo.db.NextSequence()
		if err != nil {
			return nil, fmt.Errorf("add tweet sequence: %w", err)
		}
		tweet.Sequence = func(i int64) *int64 { return &i }(int64(seq))
	}

	data, err := json.JSON.Marshal(tweet)
	if err != nil {
		return nil, fmt.Errorf("tweet marshal: %w", err)
	}

	key, err := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(*tweet.TweetId).Build()
	if err != nil {
		return nil, fmt.Errorf("build timeline key: %w", err)
	}
	return &tweet, repo.db.Set(key, data)
}

// Get retrieves a tweet by its ID
func (repo *TweetRepo) Get(userID, tweetID string) (*server.Tweet, error) {
	key, err := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(tweetID).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var tweet server.Tweet
	err = json.JSON.Unmarshal(data, &tweet)
	if err != nil {
		return nil, err
	}
	return &tweet, nil
}

// Delete removes a tweet by its ID
func (repo *TweetRepo) Delete(userID, tweetID string) error {
	key, err := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userID).AddTweetId(tweetID).Build()
	if err != nil {
		return err
	}
	return repo.db.Delete(key)
}

func (repo *TweetRepo) List(userId string) ([]server.Tweet, error) {
	key, err := storage.NewPrefixBuilder(TweetsRepoName).AddUserId(userId).Build()
	if err != nil {
		return nil, err
	}

	tweets := make([]server.Tweet, 0, 20)
	err = repo.db.IterateKeysValues(key, func(key string, value []byte) error {
		var tweet server.Tweet
		err := json.JSON.Unmarshal(value, &tweet)
		if err != nil {
			return err
		}
		tweets = append(tweets, tweet)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tweets, nil
}
