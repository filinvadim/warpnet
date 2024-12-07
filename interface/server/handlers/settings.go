package handlers

import (
	"errors"
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	client "github.com/filinvadim/dWighter/node-client"
	"github.com/labstack/echo/v4"
	"net"
	"net/http"
	"strings"
)

type SettingsController struct {
	cli *client.NodeClient
}

func NewSettingsController(cli *client.NodeClient) *SettingsController {
	return &SettingsController{cli: cli}
}

func (c *SettingsController) PostV1ApiNodesSettings(ctx echo.Context, params api_gen.PostV1ApiNodesSettingsParams) error {
	var req domain_gen.AddSettingRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, domain_gen.Error{Message: err.Error()})
	}
	switch req.Name {
	case domain_gen.NodeAddresses:
		ips, err := extractIPAddresses(req.Value)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, err.Error())
		}

		err = c.cli.SendNewHosts(
			config.InternalNodeAddress.String(),
			domain_gen.NewSettingsHostsEvent{Hosts: ips},
		)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Message: err.Error()})
		}
	default:
		return ctx.JSON(http.StatusBadRequest, domain_gen.Error{Message: "unknown setting name"})
	}
	return ctx.NoContent(http.StatusOK)
}

func extractIPAddresses(addresses string) ([]string, error) {
	splitted := strings.Split(addresses, ",")
	for _, address := range splitted {
		ip := net.ParseIP(address)
		if ip == nil {
			return nil, errors.New("invalid address")
		}
	}
	return splitted, nil
}

func (c *SettingsController) GetV1ApiNodesSettings(ctx echo.Context, params api_gen.GetV1ApiNodesSettingsParams) (err error) {
	var resp any

	switch params.Name {
	case domain_gen.NodeAddresses:
		resp, err = c.cli.GetHosts(
			config.InternalNodeAddress.String(),
			domain_gen.GetSettingsHostsEvent{
				Cursor: params.Cursor,
				Limit:  params.Limit,
			},
		)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Message: err.Error()})
		}
	default:
		return ctx.JSON(http.StatusBadRequest, domain_gen.Error{Message: "unknown setting name"})
	}
	return ctx.JSON(http.StatusOK, resp)
}
