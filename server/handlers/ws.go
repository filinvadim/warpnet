package handlers

import (
	"context"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	event "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/websocket"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

type WSController struct {
	upgrader *websocket.EncryptedUpgrader
	auth     *auth.AuthService
	l        echo.Logger
	ctx      context.Context
	conf     config.Config

	client *node.WarpNode
}

func NewWSController(
	conf config.Config,
	auth *auth.AuthService,
) *WSController {

	return &WSController{
		websocket.NewEncryptedUpgrader(), auth, nil, nil, conf, nil,
	}
}

func (c *WSController) WebsocketUpgrade(ctx echo.Context) (err error) {
	ctx.Logger().Infof("websocket upgrade request: %s", ctx.Request().URL.Path)

	c.upgrader.OnMessage(c.handle)

	c.l = ctx.Logger()
	c.ctx = ctx.Request().Context()

	err = c.upgrader.UpgradeConnection(ctx.Response(), ctx.Request())
	if err != nil {
		ctx.Logger().Errorf("websocket upgrader: %v", err)
	}

	return nil
}

func (c *WSController) handle(msg []byte) (_ []byte, err error) {
	var (
		wsMsg    api.BaseWSRequest
		response any
	)
	if err := json.JSON.Unmarshal(msg, &wsMsg); err != nil {
		return nil, err
	}
	if wsMsg.MessageId == "" {
		c.l.Warn("websocket request: missing message_id")
	}

	value, err := wsMsg.Data.ValueByDiscriminator()
	if err != nil {
		return nil, err
	}

	switch value.(type) {
	case event.LoginEvent:
		loginResp, err := c.auth.AuthLogin(c.l, value.(event.LoginEvent))
		if err != nil {
			return nil, err
		}
		c.upgrader.SetNewSalt(loginResp.Data.Token) // make conn more secure after successful auth
		loginResp.MessageId = wsMsg.MessageId
		response = loginResp

		c.client, err = node.NewClientNode(c.ctx, loginResp.Data.User.NodeId, c.conf)
		if err != nil {
			log.Printf("create node client: %v", err)
		}
	case event.LogoutEvent:
		defer c.client.Stop()
		defer func() {
			if err := c.upgrader.Close(); err != nil {
				c.l.Errorf("upgrader close: %v", err)
			}
		}()
		err = c.auth.AuthLogout()
		return nil, err
	default:
		if !c.auth.IsAuthenticated() {
			response = api.ErrorResponse{Data: &api.ErrorData{Code: http.StatusUnauthorized, Message: "not authenticated"}}
			break
		}
		// TODO
	}
	if response == nil {
		return nil, nil
	}

	return json.JSON.Marshal(response)
}
