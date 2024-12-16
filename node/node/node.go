package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	domain_gen "github.com/filinvadim/warpnet/domain-gen"
	client "github.com/filinvadim/warpnet/node-client"
	"github.com/filinvadim/warpnet/node/server"
	"log"
	"os"
	"time"

	"github.com/filinvadim/warpnet/database"
)

type NodeServer interface {
	Start() error
	Stop() error
}

type NodeService struct {
	ctx    context.Context
	server NodeServer
	client *client.NodeClient

	nodeRepo *database.NodeRepo
	authRepo *database.AuthRepo
	userRepo *database.UserRepo

	stopChan chan struct{}
}

func NewNodeService(
	ctx context.Context,
	ownIP string,
	db *storage.DB,
	interrupt chan os.Signal,
) (*NodeService, error) {

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)
	replyRepo := database.NewRepliesRepo(db)

	cli, err := client.NewNodeClient(ctx)
	if err != nil {
		return nil, err
	}
	handler, err := server.NewNodeHandler(
		ownIP,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		timelineRepo,
		followRepo,
		replyRepo,
		cli,
		interrupt,
	)
	if err != nil {
		return nil, fmt.Errorf("node handler: %w", err)
	}
	srv, err := server.NewNodeServer(ctx, handler)
	if err != nil {
		return nil, fmt.Errorf("node server: %w", err)
	}

	return &NodeService{
		ctx, srv, cli, nodeRepo,
		authRepo, userRepo, make(chan struct{}),
	}, nil
}

func (ds *NodeService) Run() {
	go func() {
		if err := ds.server.Start(); err != nil {
			log.Fatalf("node server: %v", err)
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
			nodes, _, err := ds.nodeRepo.List(nil, nil)
			if err != nil {
				log.Fatalln(err)
			}
			owner, err := ds.authRepo.Owner()
			if err != nil {
				log.Fatalln(err)
			}
			ownUser, err := ds.userRepo.Get(owner)
			if err != nil {
				log.Fatalln(err)
			}
			ownNode := ds.nodeRepo.OwnNode()

			for _, n := range nodes {
				err = ds.client.Ping(n.Host, domain_gen.PingEvent{
					Nodes:     nodes,
					DestHost:  &n.Host,
					OwnerInfo: &ownUser,
					OwnerNode: ownNode,
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
