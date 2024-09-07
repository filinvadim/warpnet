package handlers

import (
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
)

type TweetController struct {
	timelineRepo *database.TimelineRepo
	tweetRepo    *database.TweetRepo
}

func NewTweetController(timelineRepo *database.TimelineRepo, tweetRepo *database.TweetRepo) *TweetController {
	return &TweetController{timelineRepo: timelineRepo, tweetRepo: tweetRepo}
}

func (c *TweetController) PostTweets(ctx echo.Context) error {
	var t api.Tweet
	err := ctx.Bind(&t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	userID := t.UserId
	tweet, err := c.tweetRepo.Create(userID, t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	err = c.timelineRepo.AddTweetToTimeline(userID, *tweet)
	if err != nil {
		_ = c.tweetRepo.Delete(userID, *tweet.TweetId)
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// TODO broadcast

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsTimelineUserId returns a user's timeline (tweets)
func (c *TweetController) GetTweetsTimelineUserId(ctx echo.Context, userId string, params api.GetTweetsTimelineUserIdParams) error {

	tweets, nextCursor, err := c.timelineRepo.GetTimeline(userId, params.Limit, params.Cursor)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	response := api.TimelineResponse{
		Tweets: tweets,
		Cursor: nextCursor,
	}

	return ctx.JSON(http.StatusOK, response)
}

// GetTweetsUserIdTweetId returns a specific tweet by userId and tweetId
func (c *TweetController) GetTweetsUserIdTweetId(ctx echo.Context, userId string, tweetId string) error {
	tweet, err := c.tweetRepo.Get(userId, tweetId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsUserId returns all tweets by a specific user
func (c *TweetController) GetTweetsUserId(ctx echo.Context, userId string) error {
	tweets, err := c.tweetRepo.List(userId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweets)
}
