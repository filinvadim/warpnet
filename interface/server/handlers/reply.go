package handlers

import (
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	client "github.com/filinvadim/dWighter/node-client"
	"github.com/labstack/echo/v4"
	"net/http"
)

type ReplyController struct {
	cli *client.NodeClient
}

func NewReplyController(
	cli *client.NodeClient,
) *ReplyController {
	return &ReplyController{cli}
}

func (rc *ReplyController) PostV1ApiTweetsReplies(ctx echo.Context, params api_gen.PostV1ApiTweetsRepliesParams) error {
	if rc == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var t domain_gen.Tweet
	err := ctx.Bind(&t)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "bind" + err.Error()})
	}

	tweet, err := rc.cli.BroadcastNewReply(
		config.InternalNodeAddress.String(),
		domain_gen.NewReplyEvent{Tweet: &t},
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, tweet)
}

func (rc *ReplyController) GetV1ApiTweetsRepliesRootTweetIdParentReplyId(ctx echo.Context, rootTweetId string, parentTweetId string, params api_gen.GetV1ApiTweetsRepliesRootTweetIdParentReplyIdParams) error {
	if rc == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	repliesTree, err := rc.cli.SendGetAllReplies(
		config.InternalNodeAddress.String(),
		domain_gen.GetRepliesEvent{
			Cursor:        params.Cursor,
			Limit:         params.Limit,
			RootId:        rootTweetId,
			ParentReplyId: parentTweetId,
		})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, repliesTree)
}

func (rc *ReplyController) GetV1ApiTweetsRepliesRootTweetIdReplyId(ctx echo.Context, rootTweetId string, tweetId string, params api_gen.GetV1ApiTweetsRepliesRootTweetIdReplyIdParams) error {
	return ctx.NoContent(http.StatusNotImplemented)
}
