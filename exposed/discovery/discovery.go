package discovery

import (
	"context"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/exposed/client"
	"github.com/filinvadim/dWighter/exposed/server"
	"github.com/filinvadim/dWighter/json"
	"io"
	"net/http"
	"time"

	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
)

const apifyAddr = "https://api.ipify.org?format=json"

type DiscoveryServer interface {
	Start() error
	Stop() error
}

type DiscoveryCacher interface {
	AddNode(n domain_gen.Node)
	GetNodes() []domain_gen.Node
	RemoveNode(n *domain_gen.Node)
}

type DiscoveryService struct {
	ctx    context.Context
	cache  DiscoveryCacher
	server DiscoveryServer
	client server.DiscoveryRequester

	nodeRepo *database.NodeRepo
	authRepo *database.AuthRepo

	stopChan chan struct{}
}

const PresetNodeAddress = "127.0.0.1:16969"

func NewDiscoveryService(
	ctx context.Context,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	timelineRepo *database.TimelineRepo,
	loggerMw echo.MiddlewareFunc,
) (*DiscoveryService, error) {
	ip, err := getOwnIPAddress()
	if err != nil {
		fmt.Println("failed to get own ip address")
	}

	fmt.Println("YOUR OWN IP", ip)
	cli, err := client.NewDiscoveryClient(ctx)
	if err != nil {
		return nil, err
	}

	cache, err := newDiscoveryCache(nodeRepo)
	if err != nil {
		return nil, err
	}

	handler, err := server.NewDiscoveryHandler(
		ip,
		cache,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		timelineRepo,
		cli,
	)
	if err != nil {
		return nil, err
	}
	srv, err := server.NewDiscoveryServer(ctx, handler, loggerMw)
	if err != nil {
		return nil, err
	}

	return &DiscoveryService{
		ctx, cache, srv, cli, nodeRepo, authRepo, make(chan struct{}),
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
				err = ds.client.Ping(n.Host, domain_gen.PingEvent{
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

func (ds *DiscoveryService) Stop() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	close(ds.stopChan)
	return ds.server.Stop()
}

type apifyResponse struct {
	IP string `json:"ip"`
}

func getOwnIPAddress() (string, error) {
	resp, err := http.Get(apifyAddr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bt, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var ipResp apifyResponse
	err = json.JSON.Unmarshal(bt, &ipResp)
	return ipResp.IP, err
}
