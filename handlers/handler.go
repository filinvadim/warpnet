package handlers

import (
	"github.com/filinvadim/dWighter/api"
	"github.com/labstack/echo/v4"
)

type Controller struct {
}

func (c Controller) GetNodeIPAddresses(ctx echo.Context) error {
	return nil
}

func (c Controller) SendNodeIPAddresses(ctx echo.Context) error {
	return nil

}

func (c Controller) PingNode(ctx echo.Context, params api.PingNodeParams) error {
	return nil
}

func (c Controller) BroadcastMessage(ctx echo.Context) error {
	return nil
}

func (c Controller) FollowUser(ctx echo.Context) error {
	return nil

}

func (c Controller) GetUserTimeline(ctx echo.Context, userId string) error {
	return nil
}

func (c Controller) CreateTweet(ctx echo.Context) error {
	return nil
}

func (c Controller) GetUserTweets(ctx echo.Context, userId string) error {
	return nil
}

func (c Controller) UnfollowUser(ctx echo.Context) error {
	return nil
}

func (c Controller) CreateUser(ctx echo.Context) error {
	return nil
}

func (c Controller) GetUser(ctx echo.Context, userId string) error {
	return nil
}
