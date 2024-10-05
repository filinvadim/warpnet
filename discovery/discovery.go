package discovery

import (
	"context"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/api/discovery"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"time"
)

type DiscoveryServer interface {
	Start() error
	Stop() error
}

type DiscoveryCacher interface {
	AddNode(n *components.Node)
	GetNodes() []components.Node
	RemoveNode(n *components.Node)
}

type DiscoveryService struct {
	ctx    context.Context
	cache  DiscoveryCacher
	server DiscoveryServer
	client DiscoveryRequester

	nodeRepo *database.NodeRepo
	authRepo *database.AuthRepo

	stopChan chan struct{}
}

const PresetNodeAddress = "127.0.0.1:6969"

func NewDiscoveryService(
	ctx context.Context,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	loggerMw echo.MiddlewareFunc,
) (*DiscoveryService, error) {
	cli, err := newDiscoveryClient(ctx, nodeRepo)
	if err != nil {
		return nil, err
	}

	cache, err := newDiscoveryCache(nodeRepo)
	if err != nil {
		return nil, err
	}

	handler, err := newDiscoveryHandler(
		cache,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		cli,
	)
	if err != nil {
		return nil, err
	}
	server, err := newDiscoveryServer(ctx, handler, nodeRepo, loggerMw)
	if err != nil {
		return nil, err
	}

	return &DiscoveryService{
		ctx, cache, server, cli, nodeRepo, authRepo, make(chan struct{}),
	}, nil
}

func (ds *DiscoveryService) Run() error {
	go ds.server.Start()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ds.ctx.Done():
			return nil
		case <-ds.stopChan:
			return nil
		case <-ticker.C:
			nodes := ds.cache.GetNodes()
			owner, err := ds.authRepo.Owner()
			if err != nil {
				return err
			}
			ownNode := ds.nodeRepo.OwnNode()

			for _, n := range nodes {
				err = ds.client.Ping(n.Host, discovery.PingEvent{
					CachedNodes: nodes,
					DestHost:    &n.Host,
					OwnerInfo:   owner,
					OwnerNode:   ownNode,
				})
				if err != nil {
					return err
				}
			}
		}
	}
}

func (ds *DiscoveryService) Stop() error {
	close(ds.stopChan)
	return ds.server.Stop()
}
