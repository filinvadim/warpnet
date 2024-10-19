package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/dWighter/database/storage"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/node/client"
	"github.com/filinvadim/dWighter/node/server"
	"log"
	"os"
	"time"

	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
)

type NodeServer interface {
	Start() error
	Stop() error
}

type NodeCacher interface {
	AddNode(n domain_gen.Node)
	GetNodes() []domain_gen.Node
	RemoveNode(n *domain_gen.Node)
}

type NodeService struct {
	ctx    context.Context
	cache  NodeCacher
	server NodeServer
	client *client.NodeClient

	nodeRepo *database.NodeRepo
	authRepo *database.AuthRepo

	stopChan chan struct{}
}

func NewNodeService(
	ctx context.Context,
	ownIP string,
	cli *client.NodeClient,
	db *storage.DB,
	interrupt chan os.Signal,
	loggerMw echo.MiddlewareFunc,
) (*NodeService, error) {

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)

	cache, err := newNodeCache(nodeRepo)
	if err != nil {
		return nil, fmt.Errorf("node cache: %w", err)
	}

	handler, err := server.NewNodeHandler(
		ownIP,
		cache,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		timelineRepo,
		followRepo,
		cli,
		interrupt,
	)
	if err != nil {
		return nil, fmt.Errorf("node handler: %w", err)
	}
	srv, err := server.NewNodeServer(ctx, handler, loggerMw)
	if err != nil {
		return nil, fmt.Errorf("node server: %w", err)
	}

	return &NodeService{
		ctx, cache, srv, cli, nodeRepo, authRepo, make(chan struct{}),
	}, nil
}

func (ds *NodeService) Run() {
	go func() {
		if err := ds.server.Start(); err != nil {
			panic(err)
		}
	}()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ds.stopChan:
			return
		case <-ticker.C:
			nodes := ds.cache.GetNodes()
			owner, err := ds.authRepo.Owner()
			if err != nil {
				log.Fatalln(err)
			}
			ownNode := ds.nodeRepo.OwnNode()

			for _, n := range nodes {
				err = ds.client.Ping(n.Host, domain_gen.PingEvent{
					CachedNodes: nodes,
					DestHost:    &n.Host,
					OwnerInfo:   owner,
					OwnerNode:   ownNode,
				})
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (ds *NodeService) Stop() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	close(ds.stopChan)
	if err := ds.server.Stop(); err != nil {
		log.Println(err)
	}
}
