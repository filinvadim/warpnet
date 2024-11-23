package handlers

import (
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	client "github.com/filinvadim/dWighter/node/client"
	"net/http"

	"github.com/labstack/echo/v4"
)

type UserController struct {
	cli        *client.NodeClient
	owNodeHost string
}

func NewUserController(cli *client.NodeClient) *UserController {
	return &UserController{cli: cli, owNodeHost: config.InternalNodeAddress.String()}
}

func (c *UserController) PostV1ApiUsersFollow(ctx echo.Context) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var req domain_gen.FollowRequest
	err := ctx.Bind(&req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	readerId := req.ReaderId
	writerId := req.WriterId

	_, err = c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: readerId})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: writerId})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.cli.BroadcastNewFollow(c.owNodeHost, domain_gen.NewFollowEvent{Request: &domain_gen.FollowRequest{
		ReaderId: readerId,
		WriterId: writerId,
	}})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.NoContent(http.StatusCreated)
}

// PostUsersUnfollow allows a user to unfollow another user
func (c *UserController) PostV1ApiUsersUnfollow(ctx echo.Context) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var req domain_gen.FollowRequest
	err := ctx.Bind(&req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	readerId := req.ReaderId
	writerId := req.WriterId

	_, err = c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: readerId})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: writerId})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.cli.BroadcastNewUnfollow(c.owNodeHost, domain_gen.NewUnfollowEvent{Request: &domain_gen.UnfollowRequest{
		ReaderId: readerId,
		WriterId: writerId,
	}})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.NoContent(http.StatusOK)
}

func (c *UserController) PostV1ApiUsers(ctx echo.Context) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}
	var user *domain_gen.User
	err := ctx.Bind(&user)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	if user == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "user is nil"})
	}

	if user.UserId != nil {
		_, err = c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: *user.UserId})
		if err == nil {
			return ctx.JSON(http.StatusForbidden, domain_gen.Error{Code: http.StatusForbidden, Message: "user already exists"})
		}
	}

	userCreated, err := c.cli.BroadcastNewUser(c.owNodeHost, domain_gen.NewUserEvent{User: user})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, userCreated)
}

// GetUsersUserId retrieves a user by their userId
func (c *UserController) GetV1ApiUsersUserId(ctx echo.Context, userId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}

	u, err := c.cli.GetUser(c.owNodeHost, domain_gen.GetUserEvent{UserId: userId})
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, u)
}

func (c *UserController) GetV1ApiUsers(ctx echo.Context, params api_gen.GetV1ApiUsersParams) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}

	users, err := c.cli.GetUsers(c.owNodeHost)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, users)
}
