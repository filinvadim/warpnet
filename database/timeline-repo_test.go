package database_test

import (
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func setupTimelineTestDB(t *testing.T) *storage.DB {
	path := "../var/timeline"
	// Открываем базу данных в этой директории
	db := storage.New("timelinetest", "timelinetest", path, false, true, "error")

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(path)
	})

	return db
}

func TestTimelineRepo_AddTweetToTimeline(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()
	id1 := uuid.New().String()
	id2 := uuid.New().String()
	tweet1 := api.Tweet{TweetId: &id1}
	tweet2 := api.Tweet{TweetId: &id2}

	// Add two tweets with different timestamps
	err := repo.AddTweetToTimeline(userID, tweet1)
	assert.NoError(t, err)

	err = repo.AddTweetToTimeline(userID, tweet2)
	assert.NoError(t, err)

	// Verify that both tweets were added
	tweets, err := repo.GetTimeline(userID)
	assert.NoError(t, err)
	assert.Len(t, tweets, 2)
	assert.Contains(t, tweets, tweet1)
	assert.Contains(t, tweets, tweet2)
}

func TestTimelineRepo_GetTimeline(t *testing.T) {
	db := setupTimelineTestDB(t)
	repo := database.NewTimelineRepo(db)

	userID := uuid.New().String()
	tweetID1 := uuid.New().String()
	tweetID2 := uuid.New().String()
	tweetID3 := uuid.New().String()

	tweet1 := api.Tweet{TweetId: &tweetID1}
	tweet2 := api.Tweet{TweetId: &tweetID2}
	tweet3 := api.Tweet{TweetId: &tweetID3}
	// Add three tweets
	err := repo.AddTweetToTimeline(userID, tweet1)
	assert.NoError(t, err)
	err = repo.AddTweetToTimeline(userID, tweet2)
	assert.NoError(t, err)
	err = repo.AddTweetToTimeline(userID, tweet3)
	assert.NoError(t, err)

	// Retrieve all tweets
	tweets, err := repo.GetTimeline(userID)
	assert.NoError(t, err)
	assert.Len(t, tweets, 3)
	assert.Contains(t, tweets, tweet1)
	assert.Contains(t, tweets, tweet2)
	assert.Contains(t, tweets, tweet3)
}
