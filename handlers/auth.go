package handlers

import (
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
)

type AuthController struct {
	authRepo  *database.AuthRepo
	interrupt chan os.Signal
}

func NewAuthController(authRepo *database.AuthRepo, interrupt chan os.Signal) *AuthController {
	return &AuthController{authRepo, interrupt}
}

func (c *AuthController) PostAuthLogin(ctx echo.Context) error {
	var req server.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	hashSum := crypto.ConvertToSHA256([]byte(req.Password + req.Username)) // aaaa + vadim
	if err := c.authRepo.InitWithPassword(hashSum); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	u, err := c.authRepo.SetOwner(&server.User{
		Birthdate:    nil,
		CreatedAt:    nil,
		Description:  nil,
		Followed:     nil,
		FollowedNum:  nil,
		Followers:    nil,
		FollowersNum: nil,
		Link:         nil,
		Location:     nil,
		MyReferrals:  nil,
		ReferredBy:   nil,
		UserId:       nil,
		Username:     req.Username,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// TODO broadcast id

	return ctx.JSON(http.StatusOK, u)
}

func (c *AuthController) PostAuthLogout(ctx echo.Context) error {
	c.interrupt <- os.Interrupt
	return ctx.NoContent(http.StatusOK)
}
