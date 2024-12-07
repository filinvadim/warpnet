package handlers

import (
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/interface/api-gen"
	client "github.com/filinvadim/dWighter/node-client"
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
	return &AuthController{cli, config.InternalNodeAddress.String()}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	var req domain_gen.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	authResp, err := c.cli.SendLogin(c.discoveryHost, domain_gen.LoginEvent{
		Password: req.Password,
		Username: req.Username,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx.Response().Header().Set(config.SessionTokenName, authResp.Token)
	return ctx.JSON(http.StatusOK, authResp)
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context, params api.PostV1ApiAuthLogoutParams) error {
	err := c.cli.SendLogout(c.discoveryHost, domain_gen.LogoutEvent{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.NoContent(http.StatusOK)
}
