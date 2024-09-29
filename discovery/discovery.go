package discovery

import (
	"context"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
)

type DiscoveryServer interface {
	Start() error
}

const PresetNodeAddress = "127.0.0.1:6969"

func RunDiscoveryService(
	ctx context.Context,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	loggerMw echo.MiddlewareFunc,
) error {
	u, err := authRepo.GetOwner()
	if err != nil {
		return err
	}

	nodeId := u.NodeId.String()

	cli, err := newDiscoveryClient(ctx, nodeId)
	if err != nil {
		return err
	}

	handler, err := newDiscoveryHandler(
		u,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		cli,
	)
	if err != nil {
		return err
	}
	server, err := newDiscoveryServer(ctx, handler, nodeId, loggerMw)
	if err != nil {
		return err
	}

	return server.Start()
}
