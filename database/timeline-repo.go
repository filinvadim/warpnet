package database

import (
	"errors"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sort"
	"time"

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

func (repo *TimelineRepo) AddTweetToTimeline(userID string, tweet domain_gen.Tweet) error {
	if userID == "" {
		return errors.New("userID cannot be blank")
	}
	if tweet.TweetId == nil {
		return fmt.Errorf("tweet id should not be nil")
	}
	if tweet.CreatedAt == nil {
		return fmt.Errorf("tweet created at should not be nil")
	}

	key, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).Build()
	if err != nil {
		return fmt.Errorf("build timeline key: %w", err)
	}

	data, err := json.JSON.Marshal(tweet)
	if err != nil {
		return fmt.Errorf("timeline marshal: %w", err)
	}
	return repo.db.Set(key, data)
}

func (repo *TimelineRepo) DeleteTweetFromTimeline(userID string, createdAt time.Time) error {
	if userID == "" {
		return errors.New("userID cannot be blank")
	}
	if createdAt.IsZero() {
		return fmt.Errorf("createdAt should not be zero")
	}
	key, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).Build()
	if err != nil {
		return err
	}

	return repo.db.Delete(key)
}

// GetTimeline retrieves a user's timeline sorted from newest to oldest
func (repo *TimelineRepo) GetTimeline(userID string, limit *uint64, cursor *string) ([]domain_gen.Tweet, string, error) {
	if userID == "" {
		return nil, "", errors.New("userID cannot be blank")
	}
	if *limit == 0 {
		limit = new(uint64)
		*limit = 20
	}

	prefix, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, "", err
	}

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
