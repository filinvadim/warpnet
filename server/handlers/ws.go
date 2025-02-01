package handlers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node/client"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"

	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/websocket"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"time"
)

type GenericStreamer interface {
	GenericStream(string, stream.WarpRoute, []byte) ([]byte, error)
	Stop()
}

type AuthServicer interface {
	AuthLogin(message event.LoginEvent) (resp event.LoginResponse, err error)
	AuthLogout() error
	IsAuthenticated() bool
}

type WSController struct {
	upgrader *websocket.EncryptedUpgrader
	auth     AuthServicer
	ctx      context.Context
	conf     config.Config

	client GenericStreamer
}

func NewWSController(
	conf config.Config,
	auth AuthServicer,
) *WSController {

	return &WSController{nil, auth, nil, conf, nil}
}

func (c *WSController) WebsocketUpgrade(ctx echo.Context) (err error) {
	ctx.Logger().Infof("websocket upgrade request: %s", ctx.Request().URL.Path)

	c.upgrader = websocket.NewEncryptedUpgrader()
	c.upgrader.OnMessage(c.handle)

	c.ctx = ctx.Request().Context()

	err = c.upgrader.UpgradeConnection(ctx.Response(), ctx.Request()) // WS listener infinite loop
	if err != nil {
		ctx.Logger().Errorf("websocket upgrader: %v", err)
	}
	_ = c.auth.AuthLogout()
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
		log.Printf("websocket request: missing message_id: %s\n", string(msg))
		return nil, fmt.Errorf("websocket request: missing message_id")
	}
	if wsMsg.Body == nil {
		log.Printf("websocket request: missing body: %s\n", string(msg))
		return nil, fmt.Errorf("websocket request: missing body")
	}

	switch wsMsg.Path {
	case event.PRIVATE_POST_LOGIN_1_0_0:
		req, err := wsMsg.Body.AsRequestBody()
		if err != nil {
			log.Printf("websocket: login as request: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		ev, err := req.AsLoginEvent()
		if err != nil {
			log.Printf("websocket: login as event: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		loginResp, err := c.auth.AuthLogin(ev)
		if err != nil {
			log.Printf("websocket: auth: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		c.upgrader.SetNewSalt(loginResp.Token) // make conn more secure after successful auth

		respBody := event.ResponseBody{}
		if err = respBody.FromLoginResponse(loginResp); err != nil {
			log.Printf("websocket: login FromLoginResponse: %v", err)
			break
		}
		msgBody := &event.Message_Body{}
		if err = msgBody.FromResponseBody(respBody); err != nil {
			log.Printf("websocket: login FromResponseBody: %v", err)
			break
		}

		clientInfo := domain.AuthNodeInfo{
			Identity: domain.Identity(loginResp),
			Version:  c.conf.Version.String(),
		}
		c.client, err = client.NewClientNode(c.ctx, clientInfo, c.conf)
		if err != nil {
			log.Printf("create node client: %v", err)
		}

		response.Body = msgBody
	case event.PRIVATE_POST_LOGOUT_1_0_0:
		defer c.client.Stop()
		defer c.upgrader.Close()
		return nil, c.auth.AuthLogout()

	default:
		if !c.auth.IsAuthenticated() {
			log.Printf("websocket: not authenticated: %s\n", string(msg))
			response = newErrorResp("not authenticated")
			break
		}
		if c.client == nil {
			response = newErrorResp("not connected to server node")
			break
		}
		if wsMsg.Body == nil {
			log.Printf("websocket: missing body: %s\n", string(msg))
			response = newErrorResp(fmt.Sprintf("missing data: %s", msg))
			break
		}
		// TODO check version
		data, err := (*wsMsg.Body).MarshalJSON()
		if err != nil {
			log.Printf("websocket: marshal: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		if wsMsg.NodeId == "" || wsMsg.Path == "" {
			log.Printf("websocket: missing node id or path: %s\n", string(msg))
			response = newErrorResp(
				fmt.Sprintf("missing path or node ID: %s", msg),
			)
			break
		}

		log.Println("WS incoming message:", string(data))
		respData, err := c.client.GenericStream(wsMsg.NodeId, stream.WarpRoute(wsMsg.Path), data)
		if err != nil {
			log.Printf("websocket: send stream: %v", err)
			response = newErrorResp(err.Error())
			break
		}
		respBody := event.ResponseBody{}
		_ = respBody.UnmarshalJSON(respData)
		response.Body = new(event.Message_Body)
		_ = response.Body.FromResponseBody(respBody)
	}
	if response.Body == nil {
		log.Println("websocket response body is empty")
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
