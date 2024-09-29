package discovery

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/dWighter/api/discovery"
	cr "github.com/filinvadim/dWighter/crypto"
	"net/http"
	"strings"
	"time"
)

type discoveryClient struct {
	ctx context.Context
	cli *discovery.ClientWithResponses
}

func newDiscoveryClient(ctx context.Context, nodeID string) (*discoveryClient, error) {
	tlsCli, err := newTLSClient(nodeID)
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

func (c *discoveryClient) Ping(host string, ping discovery.PingEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Ping
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return nil, err
	}
	return c.sendEvent(host, event)
}
func (c *discoveryClient) Pong(host string, ping discovery.PongEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Pong
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return nil, err
	}
	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewTweet(host string, t discovery.NewTweetEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Tweet
	event.Timestamp = time.Now()
	if err := event.Data.FromNewTweetEvent(t); err != nil {
		return nil, err
	}
	return c.sendEvent(host, event)

}

func (c *discoveryClient) SendNewUser(host string, u discovery.NewUserEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.User
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUserEvent(u); err != nil {
		return nil, err
	}
	return c.sendEvent(host, event)

}

func (c *discoveryClient) SendError(host string, e discovery.ErrorEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Error
	event.Timestamp = time.Now()
	if err := event.Data.FromErrorEvent(e); err != nil {
		return nil, err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewFollow(host string, f discovery.NewFollowEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Follow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewFollowEvent(f); err != nil {
		return nil, err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) SendNewUnfollow(host string, uf discovery.NewUnfollowEvent) (*discovery.Event, error) {
	event := discovery.Event{}
	event.EventType = discovery.Unfollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUnfollowEvent(uf); err != nil {
		return nil, err
	}

	return c.sendEvent(host, event)
}

func (c *discoveryClient) sendEvent(host string, event discovery.Event) (*discovery.Event, error) {
	if validateAddr(host) {
		return nil, errors.New("invalid host")
	}
	eventResp, err := c.cli.NewEventWithResponse(c.ctx, event, func(ctx context.Context, req *http.Request) error {
		req.Host = host
		return nil
	})
	if err != nil {
		return nil, err
	}

	if eventResp == nil {
		return nil, errors.New("empty event response")
	}
	if eventResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("request failed with code: %d, body: %s", eventResp.StatusCode(), eventResp.Body)
	}
	return eventResp.JSON200, eventResp.HTTPResponse.Body.Close()
}

func validateAddr(addr string) bool {
	return strings.Contains(addr, ":")
}
