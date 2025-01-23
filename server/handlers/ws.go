package handlers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/websocket"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"time"
)

type WSController struct {
	upgrader *websocket.EncryptedUpgrader
	auth     *auth.AuthService
	ctx      context.Context
	conf     config.Config

	client *node.WarpNode
}

func NewWSController(
	conf config.Config,
	auth *auth.AuthService,
) *WSController {

	return &WSController{
		websocket.NewEncryptedUpgrader(), auth, nil,
		conf, nil,
	}
}

func (c *WSController) WebsocketUpgrade(ctx echo.Context) (err error) {
	ctx.Logger().Infof("websocket upgrade request: %s", ctx.Request().URL.Path)

	c.upgrader.OnMessage(c.handle)

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
		response api.BaseWSResponse
	)
	if err := json.JSON.Unmarshal(msg, &wsMsg); err != nil {
		return nil, err
	}
	if wsMsg.MessageId == "" {
		log.Println("websocket request: missing message_id")
		return nil, fmt.Errorf("websocket request: missing message_id")
	}

	switch wsMsg.Path {
	case "/login/1.0.0":
		ev, err := wsMsg.Data.AsLoginEvent()
		if err != nil {
			response = newErrorResp(err.Error())
			break
		}
		loginResp, err := c.auth.AuthLogin(ev)
		if err != nil {
			response = newErrorResp(err.Error())
			break
		}
		c.upgrader.SetNewSalt(loginResp.Token) // make conn more secure after successful auth

		loginRespBytes, err := json.JSON.Marshal(loginResp)
		if err != nil {
			response = newErrorResp(err.Error())
			break
		}
		response.Data = loginRespBytes

		c.client, err = node.NewClientNode(c.ctx, loginResp.Owner.NodeId, c.conf)
		if err != nil {
			log.Printf("create node client: %v", err)
		}
	case "/logout/1.0.0":
		defer c.client.Stop()
		defer func() {
			if err := c.upgrader.Close(); err != nil {
				log.Printf("upgrader close: %v", err)
			}
		}()
		err = c.auth.AuthLogout()
		return nil, err

	default:
		if !c.auth.IsAuthenticated() {
			response = newErrorResp("not authenticated")
			break
		}
		if wsMsg.Data == nil {
			response = newErrorResp(fmt.Sprintf("missing data: %s", msg))
			break
		}
		data, err := (*wsMsg.Data).MarshalJSON()
		if err != nil {
			response = newErrorResp(err.Error())
			break
		}
		if wsMsg.NodeId == "" || wsMsg.Path == "" {
			response = newErrorResp(
				fmt.Sprintf("missing path or node ID: %s", msg),
			)
			break
		}
		respData, err := c.client.StreamSend(
			wsMsg.NodeId, types.WarpDiscriminator(wsMsg.Path), data,
		)
		if err != nil {
			response = newErrorResp(err.Error())
			break
		}

		response = api.BaseWSResponse{
			Data: respData,
		}
	}
	if response.Data == nil {
		return nil, nil
	}

	response.MessageId = wsMsg.MessageId
	response.NodeId = wsMsg.NodeId
	response.Path = wsMsg.Path
	response.Timestamp = time.Now()

	var buffer bytes.Buffer
	err = json.JSON.NewEncoder(&buffer).Encode(response)
	return buffer.Bytes(), nil
}

func newErrorResp(message string) api.BaseWSResponse {
	errData, _ := json.JSON.Marshal(api.ErrorData{
		Code:    http.StatusInternalServerError,
		Message: message,
	})
	return api.BaseWSResponse{
		Data: errData,
	}
}
