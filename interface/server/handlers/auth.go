package handlers

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/database"
	api "github.com/filinvadim/warpnet/interface/api-gen"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type AuthController struct {
	isAuthenticated *atomic.Bool
	authRepo        *database.AuthRepo
	userRepo        *database.UserRepo
	interrupt       chan os.Signal
	authReady       chan struct{}
}

func NewAuthController(
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	interrupt chan os.Signal,
	authReady chan struct{},
) *AuthController {
	return &AuthController{
		new(atomic.Bool),
		authRepo,
		userRepo,
		interrupt,
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

	token, err := c.authRepo.Authenticate(req.Username, req.Password)
	if err != nil {
		return ctx.JSON(
			http.StatusForbidden,
			api.Error{403, fmt.Sprintf("autenticate: %v", err)},
		)
	}

	owner, err := c.userRepo.Owner()
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		return ctx.JSON(http.StatusInternalServerError, api.Error{500, err.Error()})
	}
	if errors.Is(err, database.ErrUserNotFound) {
		err := c.userRepo.CreateOwner(database.Owner{
			CreatedAt: time.Now(),
			Username:  req.Username,
		})
		if err != nil {
			return ctx.JSON(
				http.StatusInternalServerError,
				api.Error{500, fmt.Sprintf("create owner: %v", err)},
			)
		}
		owner, _ = c.userRepo.Owner()
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

	c.authReady <- struct{}{}
	c.isAuthenticated.Store(true)
	ctx.Response().Header().Set(config.SessionTokenName, token)
	return ctx.JSON(http.StatusOK, api.LoginResponse{token, userResponse})
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context, _ api.PostV1ApiAuthLogoutParams) error {
	c.interrupt <- os.Interrupt
	return ctx.NoContent(http.StatusOK)
}
