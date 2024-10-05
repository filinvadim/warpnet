package discovery

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/dWighter/api/discovery"
	cr "github.com/filinvadim/dWighter/crypto"
	"github.com/filinvadim/dWighter/database"
	"net/http"
	"strings"
	"time"
)

type discoveryClient struct {
	ctx context.Context
	cli *discovery.ClientWithResponses
}

func newDiscoveryClient(ctx context.Context, nodeRepo *database.NodeRepo) (*discoveryClient, error) {
	tlsCli, err := newTLSClient(nodeRepo.OwnNode().Id.String())
	if err != nil {
		return nil, err
	}
	cli, err := discovery.NewClientWithResponses(PresetNodeAddress, discovery.WithHTTPClient(tlsCli))
	if err != nil {
		return nil, err
	}
	return &discoveryClient{ctx, cli}, nil
}

func newTLSClient(nodeID string) (discovery.HttpRequestDoer, error) {
	conf, err := cr.GenerateTLSConfig(nodeID) // TODO check db
	if err != nil {
		return nil, err
	}
	cli := http.DefaultClient
	tr := &http.Transport{
		TLSClientConfig: conf,
	}
	cli.Transport = tr
	cli.Timeout = 10 * time.Second
	return cli, err
}

func (c *discoveryClient) Ping(host string, ping discovery.PingEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Ping
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	return c.sendEvent(host, event)
}
func (c *discoveryClient) Pong(host string, ping discovery.PongEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Pong
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewTweet(host string, t discovery.NewTweetEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Tweet
	event.Timestamp = time.Now()
	if err := event.Data.FromNewTweetEvent(t); err != nil {
		return err
	}
	return c.sendEvent(host, event)

}

func (c *discoveryClient) SendNewUser(host string, u discovery.NewUserEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.User
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUserEvent(u); err != nil {
		return err
	}
	return c.sendEvent(host, event)

}

func (c *discoveryClient) SendError(host string, e discovery.ErrorEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Error
	event.Timestamp = time.Now()
	if err := event.Data.FromErrorEvent(e); err != nil {
		return err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewFollow(host string, f discovery.NewFollowEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Follow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewFollowEvent(f); err != nil {
		return err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewUnfollow(host string, uf discovery.NewUnfollowEvent) error {
	event := discovery.Event{}
	event.EventType = discovery.Unfollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUnfollowEvent(uf); err != nil {
		return err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) sendEvent(host string, event discovery.Event) error {
	if validateAddr(host) {
		return errors.New("invalid host")
	}
	resp, err := c.cli.NewEventWithResponse(c.ctx, event, func(ctx context.Context, req *http.Request) error {
		req.Host = host
		return nil
	})
	if err != nil {
		return err
	}

	if resp == nil {
		return errors.New("empty event response")
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("request failed with code: %d, body: %s", resp.StatusCode(), resp.Body)
	}
	return resp.HTTPResponse.Body.Close()
}

func validateAddr(addr string) bool {
	return strings.Contains(addr, ":")
}
