package handlers

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	api "github.com/filinvadim/warpnet/server/api-gen"
	"github.com/filinvadim/warpnet/server/server"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type UserPersistencyLayer interface {
	Authenticate(username, password string) (token string, err error)
	Create(user domainGen.User) (domainGen.User, error)
	Owner() (domainGen.Owner, error)
	CreateOwner(o domainGen.Owner) (err error)
}

type AuthController struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	interrupt       chan os.Signal
	nodeReady       chan string
	authReady       chan struct{}
}

func NewAuthController(
	userPersistence UserPersistencyLayer,
	interrupt chan os.Signal,
	nodeReady chan string,
	authReady chan struct{},
) *AuthController {
	return &AuthController{
		new(atomic.Bool),
		userPersistence,
		interrupt,
		nodeReady,
		authReady,
	}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	if c.isAuthenticated.Load() {
		return ctx.JSON(http.StatusBadRequest, api.Error{400, "already authenticated"})
	}
	var req api.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(
			http.StatusBadRequest, api.Error{400, fmt.Sprintf("bind: %v", err)},
		)
	}

	token, err := c.userPersistence.Authenticate(req.Username, req.Password)
	if err != nil {
		return ctx.JSON(
			http.StatusForbidden,
			api.Error{403, fmt.Sprintf("autenticate: %v", err)},
		)
	}
	c.authReady <- struct{}{}

	owner, err := c.userPersistence.Owner()
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		return ctx.JSON(http.StatusInternalServerError, api.Error{500, err.Error()})
	}
	if errors.Is(err, database.ErrUserNotFound) {
		err := c.userPersistence.CreateOwner(domainGen.Owner{
			CreatedAt: time.Now(),
			Username:  req.Username,
		})
		if err != nil {
			return ctx.JSON(
				http.StatusInternalServerError,
				api.Error{500, fmt.Sprintf("create owner: %v", err)},
			)
		}
		owner, _ = c.userPersistence.Owner()
	}
	userResponse := api.Owner{
		CreatedAt:   owner.CreatedAt,
		Description: owner.Description,
		Id:          owner.Id,
		NodeId:      owner.NodeId,
		Username:    owner.Username,
	}
	if owner.Username != req.Username {
		return ctx.JSON(
			http.StatusBadRequest,
			api.Error{400, fmt.Sprintf("user %s doesn't exist", req.Username)},
		)
	}

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.JSON(http.StatusInternalServerError, api.Error{500, "node starting is timed out"})
	case nodeId := <-c.nodeReady:
		owner.NodeId = nodeId
	}

	if err = c.userPersistence.CreateOwner(owner); err != nil {
		return ctx.JSON(
			http.StatusInternalServerError,
			api.Error{500, fmt.Sprintf("failed to update user: %v", err)},
		)
	}
	c.isAuthenticated.Store(true)
	ctx.Response().Header().Set(server.SessionTokenName, token)
	return ctx.JSON(http.StatusOK, api.LoginResponse{token, userResponse})
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context, _ api.PostV1ApiAuthLogoutParams) error {
	c.interrupt <- os.Interrupt
	return ctx.NoContent(http.StatusOK)
}
