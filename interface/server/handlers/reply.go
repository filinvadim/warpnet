package handlers

import (
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	"github.com/filinvadim/dWighter/node/client"
	"github.com/labstack/echo/v4"
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
	//TODO implement me
	return nil
}

func (rc *ReplyController) GetV1ApiTweetsRepliesRootTweetIdParentTweetId(ctx echo.Context, rootTweetId string, parentTweetId string, params api_gen.GetV1ApiTweetsRepliesRootTweetIdParentTweetIdParams) error {
	//TODO implement me
	return nil
}

func (rc *ReplyController) GetV1ApiTweetsRepliesRootTweetIdTweetId(ctx echo.Context, rootTweetId string, tweetId string, params api_gen.GetV1ApiTweetsRepliesRootTweetIdTweetIdParams) error {
	//TODO implement me
	return nil
}
