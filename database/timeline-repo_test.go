package database_test

import (
	domain_gen "github.com/filinvadim/warpnet/gen/domain-gen"
	"os"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupTimelineTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtesttimeline"
	db, closer, _ := storage.New(path, true, "storage")
	db.Run("", "")

	t.Cleanup(func() {
		closer()
		os.RemoveAll(path)
	})

	return db
}

func createTestTweet(id string, timestamp time.Time) domain_gen.Tweet {
	return domain_gen.Tweet{
		Id:        id,
		Text:      "Test content",
		UserId:    uuid.New().String(),
		CreatedAt: timestamp,
	}
}

// Test for adding and retrieving tweets with limit and cursor
func TestAddAndGetTimelineWithCursorAndLimit(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := "U"
	tweet1 := createTestTweet("tweet1", time.Now().Add(-time.Hour))
	tweet2 := createTestTweet("tweet2", time.Now().Add(-30*time.Minute))
	tweet3 := createTestTweet("tweet3", time.Now())

	// Add tweets to the timeline
	_ = repo.AddTweetToTimeline(userID, tweet1)
	_ = repo.AddTweetToTimeline(userID, tweet2)
	_ = repo.AddTweetToTimeline(userID, tweet3)

	lim := uint64(2)
	cur := ""
	// Retrieve first two tweets (should return tweet3 and tweet2)
	tweets, cursor, err := repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 2)

	assert.Equal(t, tweet3.Id, tweets[0].Id)
	assert.Equal(t, tweet2.Id, tweets[1].Id)

	// Use the cursor to retrieve the last tweet (tweet1)
	tweets, newCursor, err := repo.GetTimeline(userID, &lim, &cursor)
	assert.NoError(t, err)
	assert.Len(t, tweets, 1)
	assert.Equal(t, tweet1.Id, tweets[0].Id)
	assert.Equal(t, "", newCursor) // No more data
}

// Test for adding and deleting tweets
func TestAddAndDeleteTweetFromTimeline(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()
	tweet := createTestTweet("tweet1", time.Now().Add(-time.Hour))

	// Add a tweet to the timeline
	err := repo.AddTweetToTimeline(userID, tweet)
	assert.NoError(t, err)

	lim := uint64(1)
	cur := ""
	// Verify that the tweet was added
	tweets, _, err := repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 1)
	assert.Equal(t, tweet.Id, tweets[0].Id)

	// Delete the tweet from the timeline
	err = repo.DeleteTweetFromTimeline(userID, tweets[0].Id, tweets[0].CreatedAt)
	assert.NoError(t, err)

	// Verify that the tweet was deleted
	tweets, _, err = repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 0)
}

// Test for retrieving tweets from an empty timeline
func TestGetTimelineEmpty(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()

	lim := uint64(5)
	cur := ""
	// Retrieve tweets from an empty timeline
	tweets, cursor, err := repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 0)
	assert.Equal(t, "", cursor)
}

// Test for adding multiple tweets and ensuring limit and pagination work
func TestGetTimelineWithLargeLimit(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()
	tweet1 := createTestTweet("tweet1", time.Now().Add(-time.Hour))
	tweet2 := createTestTweet("tweet2", time.Now().Add(-30*time.Minute))
	tweet3 := createTestTweet("tweet3", time.Now())

	// Add three tweets to the timeline
	_ = repo.AddTweetToTimeline(userID, tweet1)
	_ = repo.AddTweetToTimeline(userID, tweet2)
	_ = repo.AddTweetToTimeline(userID, tweet3)

	lim := uint64(10)
	cur := ""
	// Retrieve all tweets with a large limit
	tweets, cursor, err := repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 3)
	assert.Equal(t, tweet3.Id, tweets[0].Id)
	assert.Equal(t, tweet2.Id, tweets[1].Id)
	assert.Equal(t, tweet1.Id, tweets[2].Id)
	assert.Equal(t, "", cursor)
}

// Test for invalid cursor
func TestGetTimelineInvalidCursor(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()
	tweet := createTestTweet("tweet1", time.Now().Add(-time.Hour))
	_ = repo.AddTweetToTimeline(userID, tweet)

	lim := uint64(5)
	cur := "invalid_cursor"
	// Pass an invalid cursor (should return no results)
	tweets, cursor, err := repo.GetTimeline(userID, &lim, &cur)
	assert.NoError(t, err)
	assert.Len(t, tweets, 0)
	assert.Equal(t, "", cursor)
}
