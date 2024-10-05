package discovery

import (
	"fmt"
	"github.com/filinvadim/dWighter/api/discovery"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
)

type DiscoveryRequester interface {
	Ping(host string, ping discovery.PingEvent) error
	Pong(host string, ping discovery.PongEvent) error
	SendNewTweet(host string, t discovery.NewTweetEvent) error
	SendNewUser(host string, u discovery.NewUserEvent) error
	SendError(host string, e discovery.ErrorEvent) error
	SendNewFollow(host string, f discovery.NewFollowEvent) error
	SendNewUnfollow(host string, uf discovery.NewUnfollowEvent) error
}

type discoveryHandler struct {
	nodeRepo  *database.NodeRepo
	authRepo  *database.AuthRepo
	userRepo  *database.UserRepo
	tweetRepo *database.TweetRepo
	cli       DiscoveryRequester
	cache     DiscoveryCacher
}

func newDiscoveryHandler(
	cache DiscoveryCacher,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	cli DiscoveryRequester,
) (*discoveryHandler, error) {

	return &discoveryHandler{
		nodeRepo:  nodeRepo,
		authRepo:  authRepo,
		userRepo:  userRepo,
		tweetRepo: tweetRepo,
		cli:       cli,
		cache:     cache,
	}, nil
}

func (d *discoveryHandler) NewEvent(ctx echo.Context) error {
	var receivedEvent discovery.Event
	if err := ctx.Bind(&receivedEvent); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())

	}
	if receivedEvent.Data == nil {
		return nil
	}

	switch receivedEvent.EventType {
	case discovery.Ping:
		pingEvent, err := receivedEvent.Data.AsPingEvent()
		if err != nil {
			return err
		}
		destHost := ctx.Request().Host

		if pingEvent.OwnerNode.Host == "" {
			pingEvent.OwnerNode.Host = destHost
		}
		d.cache.AddNode(pingEvent.OwnerNode)

		_, err = d.userRepo.Create(pingEvent.OwnerInfo)
		if err != nil {
			return err
		}
		_, err = d.nodeRepo.Create(pingEvent.OwnerNode)
		if err != nil {
			return err
		}

		owner, _ := d.authRepo.Owner()

		if err = d.cli.Pong(destHost, discovery.PongEvent{
			CachedNodes: d.cache.GetNodes(),
			DestHost:    &destHost,
			OwnerInfo:   owner,
			OwnerNode:   d.nodeRepo.OwnNode(),
		}); err != nil {
			return err
		}
		for _, n := range pingEvent.CachedNodes {
			n := n
			d.cache.AddNode(&n)

			_, err = d.nodeRepo.Create(&n)
			if err != nil {
				return err
			}
		}
	case discovery.User:
		userEvent, err := receivedEvent.Data.AsNewUserEvent()
		if err != nil {
			return err
		}
		_, err = d.userRepo.Create(userEvent.User)
		if err != nil {
			return err
		}
		for _, n := range d.cache.GetNodes() {
			if err = d.cli.SendNewUser(n.Host, userEvent); err != nil {
				return err
			}
		}
	case discovery.Tweet:
		tweetEvent, err := receivedEvent.Data.AsNewTweetEvent()
		if err != nil {
			return err
		}
		_, err = d.tweetRepo.Create(tweetEvent.Tweet.UserId, tweetEvent.Tweet)
		if err != nil {
			return err
		}
		for _, n := range d.cache.GetNodes() {
			if err = d.cli.SendNewTweet(n.Host, tweetEvent); err != nil {
				return err
			}
		}
	case discovery.Follow, discovery.Unfollow:
	case discovery.Error:
		if receivedEvent.Data == nil {
			return nil
		}
		errEvent, err := receivedEvent.Data.AsErrorEvent()
		if err != nil {
			return err
		}

		fmt.Printf("received event: %d %s", errEvent.Code, errEvent.Message)
	default:
		if err := d.cli.SendError(ctx.Request().Host, discovery.ErrorEvent{
			Code:    -1,
			Message: "unknown event",
		}); err != nil {
			return err
		}
	}
	return nil
}
