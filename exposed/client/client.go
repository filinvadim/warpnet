package client

import (
	"context"
	"errors"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/exposed/discovery"
	discovery_gen "github.com/filinvadim/dWighter/exposed/discovery-gen"
	"github.com/filinvadim/dWighter/json"
	"net/http"
	"strings"
	"time"

	cr "github.com/filinvadim/dWighter/crypto"
)

type DiscoveryClient struct {
	ctx context.Context
	cli *discovery_gen.ClientWithResponses
}

func NewDiscoveryClient(ctx context.Context) (*DiscoveryClient, error) {
	tlsCli, err := newTLSClient("")
	if err != nil {
		return nil, err
	}
	cli, err := discovery_gen.NewClientWithResponses(discovery.PresetNodeAddress, discovery_gen.WithHTTPClient(tlsCli))
	if err != nil {
		return nil, err
	}
	return &DiscoveryClient{ctx, cli}, nil
}

func newTLSClient(nodeID string) (discovery_gen.HttpRequestDoer, error) {
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

func (c *DiscoveryClient) Ping(host string, ping domain_gen.PingEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypePing
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	_, err := c.sendEvent(host, event)
	return err
}
func (c *DiscoveryClient) Pong(host string, ping domain_gen.PongEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypePong
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) SendNewTweet(host string, t domain_gen.NewTweetEvent) (domain_gen.Tweet, error) {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeNewTweet
	event.Timestamp = time.Now()
	if err := event.Data.FromNewTweetEvent(t); err != nil {
		return domain_gen.Tweet{}, err
	}

	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.Tweet{}, err
	}
	var tweet domain_gen.Tweet
	err = json.JSON.Unmarshal(resp, &tweet)
	return tweet, err
}

func (c *DiscoveryClient) SendNewUser(host string, u domain_gen.NewUserEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeNewUser
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUserEvent(u); err != nil {
		return err
	}
	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) SendError(host string, e domain_gen.ErrorEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeError
	event.Timestamp = time.Now()
	if err := event.Data.FromErrorEvent(e); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) SendNewFollow(host string, f domain_gen.NewFollowEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeFollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewFollowEvent(f); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) SendNewUnfollow(host string, uf domain_gen.NewUnfollowEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeUnfollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUnfollowEvent(uf); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) SendLogin(host string, l domain_gen.LoginEvent) (domain_gen.User, error) {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeLogin
	event.Timestamp = time.Now()
	if err := event.Data.FromLoginEvent(l); err != nil {
		return domain_gen.User{}, err
	}

	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.User{}, err
	}
	var authUser domain_gen.User
	err = json.JSON.Unmarshal(resp, &authUser)
	return authUser, err
}

func (c *DiscoveryClient) SendLogout(host string, l domain_gen.LogoutEvent) error {
	event := domain_gen.Event{}
	event.EventType = domain_gen.EventEventTypeLogout
	event.Timestamp = time.Now()
	if err := event.Data.FromLogoutEvent(l); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *DiscoveryClient) sendEvent(host string, event domain_gen.Event) ([]byte, error) {
	if validateAddr(host) {
		return nil, errors.New("invalid host")
	}
	resp, err := c.cli.NewEventWithResponse(c.ctx, event, func(ctx context.Context, req *http.Request) error {
		req.Host = host
		return nil
	})
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, errors.New("empty event response")
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("request failed with code: %d, body: %s", resp.StatusCode(), resp.Body)
	}

	return resp.Body, nil
}

func validateAddr(addr string) bool {
	return strings.Contains(addr, ":")
}
