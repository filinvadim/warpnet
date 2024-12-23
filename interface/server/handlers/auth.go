package handlers

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/database"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/filinvadim/warpnet/interface/api-gen"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
	"time"
)

type AuthController struct {
	authRepo  *database.AuthRepo
	userRepo  *database.UserRepo
	interrupt chan os.Signal
}

func NewAuthController(
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	interrupt chan os.Signal,
) *AuthController {
	return &AuthController{
		authRepo,
		userRepo,
		interrupt,
	}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	var req domainGen.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	token, err := c.authRepo.Authenticate(req.Username, req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	owner, err := c.userRepo.Owner()
	if errors.Is(err, database.ErrUserNotFound) {
		err := c.userRepo.CreateOwner(database.Owner{
			CreatedAt: time.Now(),
			Username:  req.Username,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	userResponse := domainGen.User{
		CreatedAt:   owner.CreatedAt,
		Description: owner.Description,
		Id:          owner.Id,
		NodeId:      owner.NodeId,
		Username:    owner.Username,
	}
	if owner.Username != req.Username {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("user %s doesn't exist", req.Username))
	}
	if owner.Username == req.Username {
		return ctx.JSON(http.StatusOK, domainGen.LoginResponse{token, userResponse})
	}

	ctx.Response().Header().Set(config.SessionTokenName, token)
	return ctx.JSON(http.StatusOK, domainGen.LoginResponse{token, userResponse})
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context, _ api.PostV1ApiAuthLogoutParams) error {
	c.interrupt <- os.Interrupt
	return ctx.NoContent(http.StatusOK)
}
