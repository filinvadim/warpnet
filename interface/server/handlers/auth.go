package handlers

import (
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/node/client"
	"github.com/filinvadim/dWighter/node/server"
	"github.com/labstack/echo/v4"
	"net/http"
)

type AuthController struct {
	cli           *client.NodeClient
	discoveryHost string
}

func NewAuthController(
	cli *client.NodeClient,
) *AuthController {
	return &AuthController{cli, "http://localhost" + server.DefaultDiscoveryPort}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	fmt.Println(ctx.Request().URL.String(), "????????????????")
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
