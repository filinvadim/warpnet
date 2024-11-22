package database

import (
	"errors"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"math"
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
	if tweet.TweetId == nil {
		return fmt.Errorf("tweet id should not be nil")
	}
	if tweet.CreatedAt == nil {
		return fmt.Errorf("tweet created at should not be nil")
	}
	if tweet.Sequence == nil {
		tweet.Sequence = new(int64)
		newSeqNum, err := repo.db.NextSequence()
		if err != nil {
			return fmt.Errorf("add timeline tweet sequence: %w", err)
		}
		*tweet.Sequence = math.MaxInt64 - int64(newSeqNum)
	}

	key, err := storage.NewPrefixBuilder(TimelineRepoName).
		AddUserId(userID).
		AddReverseTimestamp(*tweet.CreatedAt).
		AddSequence(*tweet.Sequence).
		Build()
	if err != nil {
		return fmt.Errorf("build timeline key: %w", err)
	}

	data, err := json.JSON.Marshal(tweet)
	if err != nil {
		return fmt.Errorf("timeline marshal: %w", err)
	}
	return repo.db.Set(key, data)
}

func (repo *TimelineRepo) DeleteTweetFromTimeline(userID string, createdAt time.Time, seqNum int64) error {
	if createdAt.IsZero() {
		return fmt.Errorf("createdAt should not be zero")
	}
	if seqNum == 0 {
		return fmt.Errorf("seqNum should not be zero")
	}
	key, err := storage.NewPrefixBuilder(TimelineRepoName).
		AddUserId(userID).
		AddReverseTimestamp(createdAt).
		AddSequence(seqNum).
		Build()
	if err != nil {
		return err
	}

	return repo.db.Delete(key)
}

// GetTimeline retrieves a user's timeline sorted from newest to oldest
func (repo *TimelineRepo) GetTimeline(userID string, limit *uint64, cursor *string) ([]domain_gen.Tweet, string, error) {
	if limit == nil {
		limit = new(uint64)
		*limit = 20
	}
	if *limit == 0 {
		limit = new(uint64)
		*limit = 20
	}
	tweets := make([]domain_gen.Tweet, 0, *limit)
	prefix, err := storage.NewPrefixBuilder(TimelineRepoName).AddUserId(userID).Build()
	if err != nil {
		return nil, "", err
	}

	var lastKey string
	if cursor != nil && *cursor != "" {
		prefix = *cursor
	}

	err = repo.db.IterateKeysValues(prefix, func(key string, value []byte) error {
		if len(tweets) >= int(*limit) {
			lastKey = key
			return storage.ErrStopIteration
		}
		if !IsValidForPrefix(key, prefix) {
			return nil
		}

		var t domain_gen.Tweet
		if err = json.JSON.Unmarshal(value, &t); err != nil {
			return err
		}
		tweets = append(tweets, t)
		return nil
	})
	if errors.Is(err, storage.ErrStopIteration) || err == nil {
		if len(tweets) < int(*limit) {
			lastKey = ""
		}
		return tweets, lastKey, nil
	}
	if err != nil {
		return nil, "", err
	}

	sort.SliceStable(tweets, func(i, j int) bool {
		return tweets[i].CreatedAt.After(*tweets[j].CreatedAt)
	})

	return tweets, lastKey, nil
}

func IsValidForPrefix(key string, prefix string) bool {
	isValid := len(key) >= len(prefix) && key[:len(prefix)] == prefix
	return isValid
}
