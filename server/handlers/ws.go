package handlers

import (
	"context"
	"github.com/filinvadim/warpnet/config"
	event "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/filinvadim/warpnet/server/auth"
	client "github.com/filinvadim/warpnet/server/node-client"
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

	client *client.ClientNode
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
		wsMsg    api.WebsocketMessage
		response any
	)
	if err := json.JSON.Unmarshal(msg, &wsMsg); err != nil {
		return nil, err
	}

	switch wsMsg.EventType {
	case "LoginEvent": // TODO
		var loginMsg event.LoginEvent
		json.JSON.Unmarshal(msg, &loginMsg)

		loginResp, err := c.auth.AuthLogin(c.l, loginMsg)
		if err != nil {
			return nil, err
		}
		c.upgrader.SetNewSalt(loginResp.Token)
		loginResp.MessageId = wsMsg.MessageId
		response = loginResp

		c.client, err = client.NewNodeClient(c.ctx, loginResp.User.NodeId, c.conf)
		if err != nil {
			log.Printf("create node client: %v", err)
		}
	case "LogoutEvent": // TODO
		defer c.client.Close()
		defer func() {
			if err := c.upgrader.Close(); err != nil {
				c.l.Errorf("upgrader close: %v", err)
			}
		}()
		err = c.auth.AuthLogout()
		return nil, err
	default:
		if !c.auth.IsAuthenticated() {
			response = api.ErrorResponse{Code: http.StatusUnauthorized, Message: "not authenticated"}
			break
		}
		// TODO
	}
	if response == nil {
		return nil, nil
	}

	return json.JSON.Marshal(response)
}
