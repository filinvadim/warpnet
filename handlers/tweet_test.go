package handlers_test

import (
	"bytes"
	"encoding/json"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// setupTest initializes a new Echo instance and test database for tweets
func setupTweetTest(t *testing.T) (*echo.Echo, *database.TweetRepo, *database.TimelineRepo, func()) {
	e := echo.New()

	path := "../var/handlertest"
	db := storage.New("tweettest", path, false, true, "error")
	tweetRepo := database.NewTweetRepo(db)
	timelineRepo := database.NewTimelineRepo(db)

	// Возвращаем echo, репозитории и функцию очистки базы данных
	cleanup := func() {
		db.Close()
		os.RemoveAll(path)
		t.Log("CLEANED")
	}
	return e, tweetRepo, timelineRepo, cleanup
}

// TestPostTweet tests the creation of a new tweet
func TestPostTweet(t *testing.T) {
	e, tweetRepo, timelineRepo, cleanup := setupTweetTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewTweetController(timelineRepo, tweetRepo)

	// Пример твита
	tweet := api.Tweet{
		Content: "Hello, world!",
		UserId:  uuid.New().String(),
	}

	// Создаем HTTP запрос
	tweetJson, _ := json.Marshal(tweet)
	req := httptest.NewRequest(http.MethodPost, "/tweets", bytes.NewReader(tweetJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	// Создаем контекст Echo
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostTweets(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var createdTweet api.Tweet
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createdTweet)) {
			assert.Equal(t, tweet.Content, createdTweet.Content)
			assert.Equal(t, tweet.UserId, createdTweet.UserId)
		}
	}
}

// TestGetTweetsByUser tests fetching all tweets by a specific user
func TestGetTweetsByUser(t *testing.T) {
	e, tweetRepo, timelineRepo, cleanup := setupTweetTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewTweetController(timelineRepo, tweetRepo)

	// Пример твита
	userID := uuid.New().String()
	tweet := api.Tweet{
		Content:   "Hello, world!",
		UserId:    userID,
		CreatedAt: func(t time.Time) *time.Time { return &t }(time.Now()),
	}

	// Добавляем твит в базу данных
	tweetRepo.Create(userID, tweet)

	// Создаем HTTP запрос для получения всех твитов пользователя
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"/tweets", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.GetTweetsUserId(ctx, userID)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var tweets []api.Tweet
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tweets)) {
			assert.Len(t, tweets, 1)
			assert.Equal(t, tweet.Content, tweets[0].Content)
		}
	}
}

// TestGetSpecificTweet tests fetching a specific tweet by tweet ID
func TestGetSpecificTweet(t *testing.T) {
	e, tweetRepo, timelineRepo, cleanup := setupTweetTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewTweetController(timelineRepo, tweetRepo)

	// Пример твита
	userID := uuid.New().String()
	tweetID := uuid.New().String()
	tweet := api.Tweet{
		Content:   "Hello, world!",
		UserId:    userID,
		TweetId:   &tweetID,
		CreatedAt: func(t time.Time) *time.Time { return &t }(time.Now()),
	}

	// Добавляем твит в базу данных
	tweetRepo.Create(userID, tweet)

	// Создаем HTTP запрос для получения конкретного твита
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"/tweets/"+tweetID, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.GetTweetsUserIdTweetId(ctx, userID, tweetID)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var fetchedTweet api.Tweet
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetchedTweet)) {
			assert.Equal(t, tweet.Content, fetchedTweet.Content)
			assert.Equal(t, tweet.UserId, fetchedTweet.UserId)
			assert.Equal(t, *tweet.TweetId, *fetchedTweet.TweetId)
		}
	}
}

// TestGetTimeline tests fetching a user's timeline
func TestGetTimeline(t *testing.T) {
	e, tweetRepo, timelineRepo, cleanup := setupTweetTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewTweetController(timelineRepo, tweetRepo)

	// Пример твитов
	userID := uuid.New().String()
	tweet1 := api.Tweet{
		Content:   "First tweet",
		UserId:    userID,
		CreatedAt: func(t time.Time) *time.Time { return &t }(time.Now().Add(-time.Hour)),
	}
	tweet2 := api.Tweet{
		Content:   "Second tweet",
		UserId:    userID,
		CreatedAt: func(t time.Time) *time.Time { return &t }(time.Now()),
	}

	// Добавляем твиты в базу данных
	t1, err := tweetRepo.Create(userID, tweet1)
	assert.NoError(t, err)
	t2, err := tweetRepo.Create(userID, tweet2)
	assert.NoError(t, err)

	err = timelineRepo.AddTweetToTimeline(userID, *t1)
	assert.NoError(t, err)
	err = timelineRepo.AddTweetToTimeline(userID, *t2)
	assert.NoError(t, err)

	// Создаем HTTP запрос для получения таймлайна пользователя
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID+"/timeline", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	params := api.GetTweetsTimelineUserIdParams{
		Limit:  nil,
		Cursor: nil,
	}
	if assert.NoError(t, controller.GetTweetsTimelineUserId(ctx, userID, params)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var response api.TimelineResponse
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response)) {
			assert.Len(t, response.Tweets, 2)
			assert.Equal(t, tweet2.Content, response.Tweets[0].Content)
			assert.Equal(t, tweet1.Content, response.Tweets[1].Content)
		}
	}
}
