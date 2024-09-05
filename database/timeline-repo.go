package database

import (
	"fmt"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
)

const TimelineRepoName = "TIMELINE"

// TimelineRepo manages user timelines
type TimelineRepo struct {
	db *storage.DB
}

func NewTimelineRepo(db *storage.DB) *TimelineRepo {
	return &TimelineRepo{db: db}
}

func (repo *TimelineRepo) AddTweetToTimeline(userID string, tweet api.Tweet) error {
	if tweet.TweetId == nil {
		return fmt.Errorf("tweet id should not be nil")
	}
	key, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).AddTweetId(*tweet.TweetId).Build()
	if err != nil {
		return err
	}

	bt, err := json.JSON.Marshal(tweet)
	if err != nil {
		return err
	}
	return repo.db.Set(key, bt)

}

// GetTimeline retrieves a user's timeline sorted from newest to oldest
func (repo *TimelineRepo) GetTimeline(userID string) ([]api.Tweet, error) { // TODO limit offset
	tweets := make([]api.Tweet, 0, 10)
	prefix, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, err
	}

	err = repo.db.IterateKeysValues(prefix, func(key string, value []byte) error {
		var t api.Tweet
		json.JSON.Unmarshal(value, &t)
		tweets = append(tweets, t)
		return nil
	})

	return tweets, err
}
