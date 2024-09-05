package handlers

import (
	"github.com/filinvadim/dWighter/api"
	"github.com/labstack/echo/v4"
)

type Controller struct {
}

func (c Controller) PostBroadcast(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) PostFollow(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetNodesIpAddresses(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) PostNodesIpAddresses(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetPing(ctx echo.Context, params api.GetPingParams) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetTimelineUserId(ctx echo.Context, userId string) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) PostTweets(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetTweetsUserId(ctx echo.Context, userId string) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) PostUnfollow(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) PostUsers(ctx echo.Context) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetUsersUserId(ctx echo.Context, userId string) error {
	//TODO implement me
	panic("implement me")
}

func (c Controller) GetNodeIPAddresses(ctx echo.Context) error {
	return nil
}

func (c Controller) SendNodeIPAddresses(ctx echo.Context) error {
	return nil

}

func (c Controller) PingNode(ctx echo.Context, params api.GetPingParams) error {
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
