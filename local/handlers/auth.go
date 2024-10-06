package handlers

import (
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/exposed/client"
	"github.com/filinvadim/dWighter/exposed/server"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
)

type AuthController struct {
	cli           *client.DiscoveryClient
	interrupt     chan os.Signal
	discoveryHost string
}

func NewAuthController(
	cli *client.DiscoveryClient,
	interrupt chan os.Signal,
) *AuthController {
	return &AuthController{cli, interrupt, "localhost" + server.DefaultDiscoveryPort}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	var req domain_gen.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	u, err := c.cli.SendLogin(c.discoveryHost, domain_gen.LoginEvent{
		Password: req.Password,
		Username: req.Username,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, u)
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context) error {
	err := c.cli.SendLogout(c.discoveryHost, domain_gen.LogoutEvent{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.NoContent(http.StatusOK)
}
