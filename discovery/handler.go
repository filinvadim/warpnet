package discovery

import (
	"fmt"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/api/discovery"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
)

type DiscoveryRequester interface {
	Ping(host string, ping discovery.PingEvent) (*discovery.Event, error)
	Pong(host string, ping discovery.PongEvent) (*discovery.Event, error)
	SendNewTweet(host string, t discovery.NewTweetEvent) (*discovery.Event, error)
	SendNewUser(host string, u discovery.NewUserEvent) (*discovery.Event, error)
	SendError(host string, e discovery.ErrorEvent) (*discovery.Event, error)
	SendNewFollow(host string, f discovery.NewFollowEvent) (*discovery.Event, error)
	SendNewUnfollow(host string, uf discovery.NewUnfollowEvent) (*discovery.Event, error)
}

type discoveryHandler struct {
	nodeRepo   *database.NodeRepo
	authRepo   *database.AuthRepo
	userRepo   *database.UserRepo
	tweetRepo  *database.TweetRepo
	cli        DiscoveryRequester
	hostsCache map[string]struct{}
	ownNodeId  string
}

func newDiscoveryHandler(
	ownNodeId string,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	cli DiscoveryRequester,
) (*discoveryHandler, error) {
	hosts := map[string]struct{}{defaultDiscoveryPort: {}}
	nodes, err := nodeRepo.List()
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		h := n.Ip + ":" + n.Port
		hosts[h] = struct{}{}
	}
	return &discoveryHandler{
		nodeRepo:   nodeRepo,
		authRepo:   authRepo,
		userRepo:   userRepo,
		tweetRepo:  tweetRepo,
		cli:        cli,
		hostsCache: hosts,
		ownNodeId:  ownNodeId,
	}, nil
}

func (d *discoveryHandler) NewEvent(ctx echo.Context) error {
	var event discovery.Event
	if err := ctx.Bind(&event); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())

	}
	if event.Data == nil {
		return nil
	}

	switch event.EventType {
	case discovery.Ping:
		pingEvent, err := event.Data.AsPingEvent()
		if err != nil {
			return err
		}
		d.hostsCache[ctx.Request().Host] = struct{}{}

		_, err = d.userRepo.Create(pingEvent.OwnerInfo)
		if err != nil {
			return err
		}
		_, err = d.nodeRepo.Create(pingEvent.OwnerNode)
		if err != nil {
			return err
		}

		_, err = d.cli.Pong(ctx.Request().Host, discovery.PongEvent{
			CachedNodes: nil,
			DestIp:      nil,
			OwnerInfo:   components.User{},
			OwnerNode:   components.Node{},
		}) // my own node
		if err != nil {
			return err
		}
		for _, n := range pingEvent.CachedNodes {
			host := n.Ip + ":" + n.Port
			d.hostsCache[host] = struct{}{}

			_, err = d.nodeRepo.Create(n)
			if err != nil {
				return err
			}
		}
	case discovery.User:
		userEvent, err := event.Data.AsNewUserEvent()
		if err != nil {
			return err
		}
		_, err = d.userRepo.Create(userEvent.User)
		if err != nil {
			return err
		}
		for host := range d.hostsCache {
			_, err = d.cli.SendNewUser(host, userEvent)
			if err != nil {
				return err
			}
		}
	case discovery.Tweet:
		tweetEvent, err := event.Data.AsNewTweetEvent()
		if err != nil {
			return err
		}
		_, err = d.tweetRepo.Create(tweetEvent.Tweet.UserId, tweetEvent.Tweet)
		if err != nil {
			return err
		}
		for host := range d.hostsCache {
			_, err = d.cli.SendNewTweet(host, tweetEvent)
			if err != nil {
				return err
			}
		}
	case discovery.Follow, discovery.Unfollow:
	case discovery.Error:
		fmt.Println(event)
	default:
		_, err := d.cli.SendError(ctx.Request().Host, discovery.ErrorEvent{
			Code:    -1,
			Message: "unknown event",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
