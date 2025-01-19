package database_test

import (
	"fmt"
	domain_gen "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/logger"
	"os"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupTweetTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtesttweet"
	// Открываем базу данных в этой директории
	l := logger.NewUnifiedLogger("debug", true)
	db, _, _ := storage.New(path, true, "/storage", l)
	db.Run("", "")

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

	tweet := domain_gen.Tweet{
		Id: tweetID,
	}

	id := uuid.New().String()
	now := time.Now()

	user := domain_gen.User{
		Username:  "User",
		Id:        id,
		CreatedAt: now,
	}

	_, err := repo.Create(user.Id, tweet)

	assert.NoError(t, err)

	// Проверяем, что пользователь был корректно создан
	retrievedTweet, err := repo.Get(user.Id, tweetID)
	assert.NoError(t, err)
	assert.Equal(t, tweet.Id, retrievedTweet.Id)
}

func TestTweetRepo_Get(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	tweetID := uuid.New().String()
	tweet := domain_gen.Tweet{
		Id: tweetID,
	}
	id := uuid.New().String()

	now := time.Now()

	user := domain_gen.User{
		Username:  "User",
		Id:        id,
		CreatedAt: now,
	}

	_, err := repo.Create(user.Id, tweet)
	assert.NoError(t, err)

	// Проверяем, что пользователь может быть получен
	retrievedTweet, err := repo.Get(user.Id, tweetID)
	assert.NoError(t, err)
	assert.Equal(t, tweet.Id, retrievedTweet.Id)
}

func TestTweetRepo_Delete(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	tweetID := uuid.New().String()
	tweet := domain_gen.Tweet{
		Id: tweetID,
	}
	id := uuid.New().String()

	user := domain_gen.User{
		Username: "User",
		Id:       id,
	}

	_, err := repo.Create(user.Id, tweet)
	assert.NoError(t, err)

	err = repo.Delete(user.Id, tweetID)
	assert.NoError(t, err)

	// Проверяем, что пользователь был удален
	retrievedTweet, err := repo.Get(user.Id, tweetID)
	assert.Error(t, err)
	assert.Nil(t, retrievedTweet)
}

func TestTweetRepo_List(t *testing.T) {
	db := setupTweetTestDB(t)
	repo := database.NewTweetRepo(db)

	id1 := "1"
	id2 := "2"
	tweet1 := domain_gen.Tweet{
		Id: id1,
	}
	tweet2 := domain_gen.Tweet{
		Id: id2,
	}
	id := uuid.New().String()

	user := domain_gen.User{
		Username: "User",
		Id:       id,
	}

	_, err := repo.Create(user.Id, tweet1)
	assert.NoError(t, err)
	_, err = repo.Create(user.Id, tweet2)
	assert.NoError(t, err)

	tweets, _, err := repo.List(user.Id, nil, nil)
	assert.NoError(t, err)
	assert.Len(t, tweets, 2)

	fmt.Println(tweets, "TWEETS!")

	// Проверяем, что все пользователи корректно получены
	assert.Contains(t, tweets, tweet1)
	assert.Contains(t, tweets, tweet2)
}
