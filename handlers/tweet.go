package handlers

import (
	"net/http"

	"github.com/filinvadim/dWighter/api/api"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
)

type TweetController struct {
	timelineRepo *database.TimelineRepo
	tweetRepo    *database.TweetRepo
}

func NewTweetController(timelineRepo *database.TimelineRepo, tweetRepo *database.TweetRepo) *TweetController {
	return &TweetController{timelineRepo: timelineRepo, tweetRepo: tweetRepo}
}

func (c *TweetController) PostV1ApiTweets(ctx echo.Context) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var t components.Tweet
	err := ctx.Bind(&t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "bind" + err.Error()})
	}

	userID := t.UserId
	tweet, err := c.tweetRepo.Create(userID, &t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "tweet:" + err.Error()})
	}
	err = c.timelineRepo.AddTweetToTimeline(userID, *tweet)
	if err != nil {
		_ = c.tweetRepo.Delete(userID, *tweet.TweetId)
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "timeline:" + err.Error()})
	}

	// TODO broadcast

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsTimelineUserId returns a user's timeline (tweets)
func (c *TweetController) GetV1ApiTweetsTimelineUserId(ctx echo.Context, userId string, params api.GetV1ApiTweetsTimelineUserIdParams) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweets, nextCursor, err := c.timelineRepo.GetTimeline(userId, params.Limit, params.Cursor)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	response := api.TimelineResponse{
		Tweets: tweets,
		Cursor: nextCursor,
	}

	return ctx.JSON(http.StatusOK, response)
}

// GetTweetsUserIdTweetId returns a specific tweet by userId and tweetId
func (c *TweetController) GetV1ApiTweetsUserIdTweetId(ctx echo.Context, userId string, tweetId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweet, err := c.tweetRepo.Get(userId, tweetId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsUserId returns all tweets by a specific user
func (c *TweetController) GetV1ApiTweetsUserId(ctx echo.Context, userId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweets, err := c.tweetRepo.List(userId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, components.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweets)
}
