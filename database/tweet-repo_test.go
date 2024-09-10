package database_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupTweetTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtesttweet"
	// Открываем базу данных в этой директории
	db := storage.New("tweettest", path, false, true, "error")

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(path)
	})

	return db
}

func TestTweetRepo_Create(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	tweetID := uuid.New().String()

	tweet := server.Tweet{
		TweetId: &tweetID,
	}

	id := uuid.New().String()
	user := server.User{
		Username: "User",
		UserId:   &id,
	}

	_, err := repo.Create(*user.UserId, tweet)

	assert.NoError(t, err)

	// Проверяем, что пользователь был корректно создан
	retrievedTweet, err := repo.Get(*user.UserId, tweetID)
	assert.NoError(t, err)
	assert.Equal(t, tweet.TweetId, retrievedTweet.TweetId)
}

func TestTweetRepo_Get(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	tweetID := uuid.New().String()
	tweet := server.Tweet{
		TweetId: &tweetID,
	}
	id := uuid.New().String()

	user := server.User{
		Username: "User",
		UserId:   &id,
	}

	_, err := repo.Create(*user.UserId, tweet)
	assert.NoError(t, err)

	// Проверяем, что пользователь может быть получен
	retrievedTweet, err := repo.Get(*user.UserId, tweetID)
	assert.NoError(t, err)
	assert.Equal(t, tweet.TweetId, retrievedTweet.TweetId)
}

func TestTweetRepo_Delete(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	tweetID := uuid.New().String()
	tweet := server.Tweet{
		TweetId: &tweetID,
	}
	id := uuid.New().String()

	user := server.User{
		Username: "User",
		UserId:   &id,
	}

	_, err := repo.Create(*user.UserId, tweet)
	assert.NoError(t, err)

	err = repo.Delete(*user.UserId, tweetID)
	assert.NoError(t, err)

	// Проверяем, что пользователь был удален
	retrievedTweet, err := repo.Get(*user.UserId, tweetID)
	assert.Error(t, err)
	assert.Nil(t, retrievedTweet)
}

func TestTweetRepo_List(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	id1 := "1"
	id2 := "2"
	tweet1 := server.Tweet{
		TweetId: &id1,
	}
	tweet2 := server.Tweet{
		TweetId: &id2,
	}
	id := uuid.New().String()

	user := server.User{
		Username: "User",
		UserId:   &id,
	}

	_, err := repo.Create(*user.UserId, tweet1)
	assert.NoError(t, err)
	_, err = repo.Create(*user.UserId, tweet2)
	assert.NoError(t, err)

	tweets, err := repo.List(*user.UserId)
	assert.NoError(t, err)
	assert.Len(t, tweets, 2)

	fmt.Println(tweets, "TWEETS!")

	// Проверяем, что все пользователи корректно получены
	assert.Contains(t, tweets, tweet1)
	assert.Contains(t, tweets, tweet2)
}
