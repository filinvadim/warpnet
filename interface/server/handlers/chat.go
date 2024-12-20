package handlers

import (
	"github.com/filinvadim/warpnet/config"
	api_gen "github.com/filinvadim/warpnet/interface/api-gen"
	client "github.com/filinvadim/warpnet/node-client"
	"github.com/labstack/echo/v4"
)

type ChatController struct {
	cli        *client.NodeClient
	owNodeHost string
}

func NewChatController(cli *client.NodeClient) *ChatController {
	return &ChatController{cli: cli, owNodeHost: config.InternalNodeAddress.String()}
}

func (c *ChatController) ListChats(ctx echo.Context, params api_gen.ListChatsParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) PostV1ApiChat(ctx echo.Context, params api_gen.PostV1ApiChatParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) GetV1ApiChatWs(ctx echo.Context, params api_gen.GetV1ApiChatWsParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) GetV1ApiChatChatId(ctx echo.Context, chatId string, params api_gen.GetV1ApiChatChatIdParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) ListMessages(ctx echo.Context, chatId string, params api_gen.ListMessagesParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) PostV1ApiChatChatIdMessage(ctx echo.Context, chatId string, params api_gen.PostV1ApiChatChatIdMessageParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) DeleteV1ApiChatChatIdMessageMessageId(ctx echo.Context, chatId string, messageId string, params api_gen.DeleteV1ApiChatChatIdMessageMessageIdParams) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChatController) GetV1ApiChatChatIdMessageMessageId(ctx echo.Context, chatId string, messageId string, params api_gen.GetV1ApiChatChatIdMessageMessageIdParams) error {
	//TODO implement me
	panic("implement me")
}
