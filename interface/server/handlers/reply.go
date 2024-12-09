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

func (rc *ReplyController) AddReply(ctx echo.Context, params api_gen.AddReplyParams) error {
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

func (rc *ReplyController) GetAllReplies(ctx echo.Context, rootReplyId string, parentReplyId string, params api_gen.GetAllRepliesParams) error {
	if rc == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	repliesTree, err := rc.cli.SendGetAllReplies(
		config.InternalNodeAddress.String(),
		domain_gen.GetRepliesEvent{
			Cursor:        params.Cursor,
			Limit:         params.Limit,
			RootId:        rootReplyId,
			ParentReplyId: parentReplyId,
		})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, repliesTree)
}

func (rc *ReplyController) GetSingleReply(
	ctx echo.Context, rootReplyId, parentReplyId, replyId string, _ api_gen.GetSingleReplyParams,
) error {
	if rc == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	reply, err := rc.cli.SendGetReply(
		config.InternalNodeAddress.String(),
		domain_gen.GetReplyEvent{
			RootId:        rootReplyId,
			ParentReplyId: parentReplyId,
			ReplyId:       replyId,
		})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, reply)
}
