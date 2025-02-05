package handlers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/gen/event-gen"

	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/websocket"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type ClientNodeStreamer interface {
	ClientStream(nodeId string, path string, data any) (_ []byte, err error)
	IsRunning() bool
}

type AuthServicer interface {
	AuthLogin(message event.LoginEvent) (resp event.LoginResponse, err error)
	AuthLogout() error
}

type WSController struct {
	upgrader *websocket.EncryptedUpgrader
	auth     AuthServicer
	ctx      context.Context
	conf     config.Config

	clientNode ClientNodeStreamer
}

func NewWSController(
	auth AuthServicer,
	clientNode ClientNodeStreamer,
) *WSController {

	return &WSController{nil, auth, nil, config.ConfigFile, clientNode}
}

func (c *WSController) WebsocketUpgrade(ctx echo.Context) (err error) {
	log.Infof("websocket upgrade request: %s", ctx.Request().URL.Path)

	c.upgrader = websocket.NewEncryptedUpgrader()
	c.upgrader.OnMessage(c.handle)

	c.ctx = ctx.Request().Context()

	err = c.upgrader.UpgradeConnection(ctx.Response(), ctx.Request()) // WS listener infinite loop
	if err != nil {
		log.Errorf("websocket upgrader: %v", err)
	}

	c.upgrader.Close()
	c.upgrader = nil

	return nil
}

func (c *WSController) handle(msg []byte) (_ []byte, err error) {
	var (
		wsMsg    event.Message
		response event.Message
	)
	if err := json.JSON.Unmarshal(msg, &wsMsg); err != nil {
		return nil, err
	}
	if wsMsg.MessageId == "" {
		log.Errorf("websocket request: missing message_id: %s\n", string(msg))
		return nil, fmt.Errorf("websocket request: missing message_id")
	}
	if wsMsg.Body == nil {
		log.Errorf("websocket request: missing body: %s\n", string(msg))
		return nil, fmt.Errorf("websocket request: missing body")
	}

	switch wsMsg.Path {
	case event.PRIVATE_POST_LOGIN:
		req, err := wsMsg.Body.AsRequestBody()
		if err != nil {
			log.Errorf("websocket: login as request: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		ev, err := req.AsLoginEvent()
		if err != nil {
			log.Errorf("websocket: login as event: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		loginResp, err := c.auth.AuthLogin(ev)
		if err != nil {
			log.Errorf("websocket: auth: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		c.upgrader.SetNewSalt(loginResp.Token) // make conn more secure after successful auth

		respBody := event.ResponseBody{}
		if err = respBody.FromLoginResponse(loginResp); err != nil {
			log.Errorf("websocket: login FromLoginResponse: %v", err)
			break
		}
		msgBody := &event.Message_Body{}
		if err = msgBody.FromResponseBody(respBody); err != nil {
			log.Errorf("websocket: login FromResponseBody: %v", err)
			break
		}
		response.Body = msgBody
	case event.PRIVATE_POST_LOGOUT:
		defer c.upgrader.Close()
		return nil, c.auth.AuthLogout()

	default:
		if c.clientNode == nil || !c.clientNode.IsRunning() {
			log.Errorf("websocket request: not connected to server node")
			response = newErrorResp("not connected to server node")
			break
		}
		if wsMsg.Body == nil {
			log.Errorf("websocket: missing body: %s\n", string(msg))
			response = newErrorResp(fmt.Sprintf("missing data: %s", msg))
			break
		}
		// TODO check version

		if wsMsg.NodeId == "" || wsMsg.Path == "" {
			log.Errorf("websocket: missing node id or path: %s\n", string(msg))
			response = newErrorResp(
				fmt.Sprintf("missing path or node ID: %s", msg),
			)
			break
		}

		log.Infof("WS incoming message: %s %s\n", wsMsg.NodeId, stream.WarpRoute(wsMsg.Path))
		respData, err := c.clientNode.ClientStream(wsMsg.NodeId, wsMsg.Path, *wsMsg.Body)
		if err != nil {
			log.Errorf("websocket: send stream: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		respBody := event.ResponseBody{}
		_ = respBody.UnmarshalJSON(respData)
		response.Body = new(event.Message_Body)
		_ = response.Body.FromResponseBody(respBody)
	}
	if response.Body == nil {
		log.Errorf("websocket response body is empty")
		return nil, nil
	}

	response.MessageId = wsMsg.MessageId
	response.NodeId = wsMsg.NodeId
	response.Path = wsMsg.Path
	response.Timestamp = time.Now()
	response.Version = c.conf.Version.String()

	var buffer bytes.Buffer
	err = json.JSON.NewEncoder(&buffer).Encode(response)
	return buffer.Bytes(), nil
}

func newErrorResp(message string) event.Message {
	errResp := event.ErrorResponse{
		Code:    http.StatusInternalServerError,
		Message: message,
	}
	respBody := event.ResponseBody{}
	_ = respBody.FromErrorResponse(errResp)
	msgBody := event.Message_Body{}
	_ = msgBody.FromResponseBody(respBody)
	resp := event.Message{
		Body: &msgBody,
	}
	return resp
}
