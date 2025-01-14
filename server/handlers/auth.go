package handlers

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	domain "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	api "github.com/filinvadim/warpnet/server/api-gen"
	"github.com/filinvadim/warpnet/server/websocket"
	"github.com/labstack/echo/v4"
	"os"
	"sync/atomic"
	"time"
)

type UserPersistencyLayer interface {
	Authenticate(username, password string) (token string, err error)
	Create(user domain.User) (domain.User, error)
	Owner() (domain.User, error)
	CreateOwner(o domain.User) (err error)
}

type AuthController struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	interrupt       chan os.Signal
	nodeReady       chan string
	authReady       chan struct{}
	l               echo.Logger
	upgrader        *websocket.EncryptedUpgrader
}

func NewAuthController(
	userPersistence UserPersistencyLayer,
	interrupt chan os.Signal,
	nodeReady chan string,
	authReady chan struct{},
) *AuthController {

	upgrader := websocket.NewEncryptedUpgrader()

	return &AuthController{
		new(atomic.Bool),
		userPersistence,
		interrupt,
		nodeReady,
		authReady,
		nil,
		upgrader,
	}
}

func (c *AuthController) WebsocketUpgrade(ctx echo.Context) (err error) {
	c.l = ctx.Logger()
	c.l.Infof("websocket upgrade request: %s", ctx.Request().URL.Path)

	c.upgrader.OnMessage(c.messageCallback)

	err = c.upgrader.UpgradeConnection(ctx.Response(), ctx.Request())
	if err != nil {
		c.l.Errorf("websocket upgrade failed: %v", err)
	}
	return nil
}

func (c *AuthController) messageCallback(msg []byte) error {
	c.l.Infof("received message: %s", string(msg))
	var wsMsg api.WebsocketMessage
	if err := json.JSON.Unmarshal(msg, &wsMsg); err != nil {
		return err
	}

	value, err := wsMsg.ValueByDiscriminator()
	if err != nil {
		return err
	}

	var response any

	switch value.(type) {
	case api.LoginMessage:
		response, err = c.authLogin(value.(api.LoginMessage))
		if err != nil {
			return err
		}
	case api.LogoutMessage:
		err = c.authLogout()
		if err != nil {
			return err
		}
	}
	if response == nil {
		return nil
	}

	bt, err := json.JSON.Marshal(response)
	if err != nil {
		return err
	}
	return c.upgrader.SendEncrypted(bt)
}

func (c *AuthController) authLogin(message api.LoginMessage) (resp api.LoginResponse, err error) {
	if c.isAuthenticated.Load() {
		c.l.Warn("already authenticated")
		return resp, errors.New("already authenticated")
	}
	c.l.Infof("authenticating user %s", message.Username)

	token, err := c.userPersistence.Authenticate(message.Username, message.Password)
	if err != nil {
		c.l.Errorf("authentication failed: %v", err)
		return resp, fmt.Errorf("authentication failed: %v", err)
	}

	owner, err := c.userPersistence.Owner()
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		c.l.Errorf("owner fetching failed: %v", err)
		return resp, err
	}
	if errors.Is(err, database.ErrUserNotFound) {
		c.l.Info("creating new owner")
		err := c.userPersistence.CreateOwner(domain.User{
			CreatedAt: time.Now(),
			Username:  message.Username,
		})
		if err != nil {
			c.l.Errorf("new owner creation failed: %v", err)
			return resp, fmt.Errorf("create owner: %v", err)

		}
		owner, _ = c.userPersistence.Owner()
	}

	if owner.Username != message.Username {
		c.l.Errorf("username mismatch: %s == %s", owner.Username, message.Username)
		return resp, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	c.authReady <- struct{}{}

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		c.l.Errorf("node startup failed: timeout")
		return resp, errors.New("node starting is timed out")
	case nodeId := <-c.nodeReady:
		owner.NodeId = nodeId
	}

	if err = c.userPersistence.CreateOwner(owner); err != nil {
		c.l.Errorf("owner update failed: %v", err)
		return resp, fmt.Errorf("failed to update user: %v", err)
	}

	c.isAuthenticated.Store(true)
	return api.LoginResponse{token, owner}, nil
}

func (c *AuthController) authLogout() error {
	c.interrupt <- os.Interrupt
	c.isAuthenticated.Store(false)
	return c.upgrader.Close()
}
