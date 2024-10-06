package handlers

import (
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"net/http"

	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
)

type UserController struct {
	userRepo   *database.UserRepo
	followRepo *database.FollowRepo
	nodeRepo   *database.NodeRepo
}

func NewUserController() *UserController {
	return &UserController{}
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

	_, err = c.userRepo.Get(readerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.userRepo.Get(writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.followRepo.Follow(readerId, writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})

	}

	// TODO broadcast

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

	_, err = c.userRepo.Get(readerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.userRepo.Get(writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.followRepo.Unfollow(readerId, writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// TODO broadcast

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

	if user.UserId != nil {
		if _, err := c.userRepo.Get(*user.UserId); err == nil {
			return ctx.JSON(http.StatusForbidden, domain_gen.Error{Code: http.StatusForbidden, Message: "user already exists"})
		}
	}

	userCreated, err := c.userRepo.Create(user)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// TODO broadcast

	return ctx.JSON(http.StatusOK, userCreated)
}

// GetUsersUserId retrieves a user by their userId
func (c *UserController) GetV1ApiUsersUserId(ctx echo.Context, userId string) error {
	if c == nil {
		return ctx.JSON(http.StatusInternalServerError, domain_gen.Error{Code: http.StatusInternalServerError, Message: "not init"})
	}

	// TODO
	return ctx.JSON(http.StatusOK, nil)
}
