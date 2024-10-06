package handlers

import (
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/exposed/client"
	"github.com/filinvadim/dWighter/exposed/server"
	api_gen "github.com/filinvadim/dWighter/local/api-gen"
	"github.com/labstack/echo/v4"
	"net/http"
)

type TweetController struct {
	cli           *client.DiscoveryClient
	discoveryHost string
}

func NewTweetController(
	cli *client.DiscoveryClient,
) *TweetController {
	return &TweetController{cli, "localhost" + server.DefaultDiscoveryPort}
}

func (c *TweetController) PostV1ApiTweets(ctx echo.Context) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var t domain_gen.Tweet
	err := ctx.Bind(&t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "bind" + err.Error()})
	}

	tweet, err := c.cli.SendNewTweet(c.discoveryHost, domain_gen.NewTweetEvent{Tweet: &t})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsTimelineUserId returns a user's timeline (tweets)
func (c *TweetController) GetV1ApiTweetsTimelineUserId(ctx echo.Context, userId string, params api_gen.GetV1ApiTweetsTimelineUserIdParams) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweets, nextCursor, err := c.timelineRepo.GetTimeline(userId, params.Limit, params.Cursor)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	response := domain_gen.TweetsResponse{
		Tweets: tweets,
		Cursor: nextCursor,
	}

	return ctx.JSON(http.StatusOK, response)
}

// GetTweetsUserIdTweetId returns a specific tweet by userId and tweetId
func (c *TweetController) GetV1ApiTweetsUserIdTweetId(ctx echo.Context, userId string, tweetId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweet, err := c.tweetRepo.Get(userId, tweetId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsUserId returns all tweets by a specific user
func (c *TweetController) GetV1ApiTweetsUserId(ctx echo.Context, userId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweets, err := c.tweetRepo.List(userId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, tweets)
}
