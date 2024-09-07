package handlers

import (
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
)

type UserController struct {
	userRepo   *database.UserRepo
	followRepo *database.FollowRepo
}

func NewUserController(userRepo *database.UserRepo, followRepo *database.FollowRepo) *UserController {
	return &UserController{userRepo, followRepo}
}

func (c *UserController) PostUsersFollow(ctx echo.Context) error {
	var req api.FollowRequest
	err := ctx.Bind(&req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	readerId := req.ReaderId
	writerId := req.WriterId

	_, err = c.userRepo.Get(readerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.userRepo.Get(writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.followRepo.Follow(readerId, writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})

	}

	// TODO broadcast

	return ctx.NoContent(http.StatusCreated)
}

// PostUsersUnfollow allows a user to unfollow another user
func (c *UserController) PostUsersUnfollow(ctx echo.Context) error {
	var req api.FollowRequest
	err := ctx.Bind(&req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	readerId := req.ReaderId
	writerId := req.WriterId

	_, err = c.userRepo.Get(readerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	_, err = c.userRepo.Get(writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	err = c.followRepo.Unfollow(readerId, writerId)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// TODO broadcast

	return ctx.NoContent(http.StatusOK)
}

func (c *UserController) PostUsers(ctx echo.Context) error {
	var user api.User
	err := ctx.Bind(&user)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	if user.UserId != nil {
		if _, err := c.userRepo.Get(*user.UserId); err == nil {
			return ctx.JSON(http.StatusForbidden, api.Error{Code: http.StatusForbidden, Message: "user already exists"})
		}
	}

	userCreated, err := c.userRepo.Create(user)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// TODO broadcast

	return ctx.JSON(http.StatusOK, userCreated)
}

// GetUsersUserId retrieves a user by their userId
func (c *UserController) GetUsersUserId(ctx echo.Context, userId string) error {
	user, err := c.userRepo.Get(userId)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, api.Error{Code: http.StatusNotFound, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, user)
}
