package handlers

import (
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	"github.com/filinvadim/dWighter/node/client"
	"github.com/labstack/echo/v4"
	"net/http"
)

type TweetController struct {
	cli        *client.NodeClient
	owNodeHost string
}

func NewTweetController(
	cli *client.NodeClient,
) *TweetController {
	return &TweetController{cli, config.InternalNodeAddress.String()}
}

func (c *TweetController) PostV1ApiTweets(ctx echo.Context, params api_gen.PostV1ApiTweetsParams) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var t domain_gen.Tweet
	err := ctx.Bind(&t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "bind" + err.Error()})
	}

	tweet, err := c.cli.BroadcastNewTweet(c.owNodeHost, domain_gen.NewTweetEvent{Tweet: &t})
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

	tweetsResp, err := c.cli.SendGetTimeline(
		c.owNodeHost, domain_gen.GetTimelineEvent{
			UserId: userId,
			Limit:  params.Limit,
			Cursor: params.Cursor,
		},
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, tweetsResp)
}

// GetTweetsUserIdTweetId returns a specific tweet by userId and tweetId
func (c *TweetController) GetV1ApiTweetsUserIdTweetId(
	ctx echo.Context, userId string, tweetId string, params api_gen.GetV1ApiTweetsUserIdTweetIdParams,
) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}

	tweet, err := c.cli.SendGetTweet(c.owNodeHost, domain_gen.GetTweetEvent{TweetId: tweetId, UserId: userId})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, tweet)
}

// GetTweetsUserId returns all tweets by a specific user
func (c *TweetController) GetV1ApiTweetsUserId(ctx echo.Context, userId string, params api_gen.GetV1ApiTweetsUserIdParams) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	tweets, err := c.cli.SendGetAllTweets(c.owNodeHost, domain_gen.GetAllTweetsEvent{UserId: userId})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, tweets)
}
