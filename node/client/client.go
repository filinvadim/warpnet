package client

import (
	"context"
	"errors"
	"fmt"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/json"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"io"
	"net/http"
	"time"

	cr "github.com/filinvadim/dWighter/crypto"
)

const (
	apifyAddr         = "https://api.ipify.org?format=txt"
	ipInfoAddr        = "https://ipinfo.io/ip"
	seeIpAddr         = "https://api.seeip.org"
	PresetNodeAddress = "127.0.0.1:16969"
)

var IPProviders = []string{apifyAddr, ipInfoAddr, seeIpAddr}

type NodeClient struct {
	ctx context.Context
	cli *node_gen.ClientWithResponses
}

func NewNodeClient(ctx context.Context) (*NodeClient, error) {
	cli, err := node_gen.NewClientWithResponses(PresetNodeAddress, func(client *node_gen.Client) error {
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &NodeClient{ctx, cli}, nil
}

func newTLSClient(nodeID string) (node_gen.HttpRequestDoer, error) {
	conf, err := cr.GenerateTLSConfig(nodeID) // TODO check db for existing certs
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

func (c *NodeClient) Ping(host string, ping domain_gen.PingEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypePing
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	_, err := c.sendEvent(host, event)
	return err
}
func (c *NodeClient) Pong(host string, ping domain_gen.PongEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypePong
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	_, err := c.sendEvent(host, event)
	return err
}

func (c *NodeClient) BroadcastNewTweet(host string, t domain_gen.NewTweetEvent) (domain_gen.Tweet, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
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

func (c *NodeClient) SendGetTweet(host string, t domain_gen.GetTweetEvent) (domain_gen.Tweet, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeGetTweet
	event.Timestamp = time.Now()
	if err := event.Data.FromGetTweetEvent(t); err != nil {
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

func (c *NodeClient) SendGetTimeline(host string, t domain_gen.GetTimelineEvent) (domain_gen.TweetsResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeGetTimeline
	event.Timestamp = time.Now()
	if err := event.Data.FromGetTimelineEvent(t); err != nil {
		return domain_gen.TweetsResponse{}, err
	}

	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.TweetsResponse{}, err
	}
	var tweetResp domain_gen.TweetsResponse
	err = json.JSON.Unmarshal(resp, &tweetResp)
	return tweetResp, err
}

func (c *NodeClient) SendGetAllTweets(host string, t domain_gen.GetAllTweetsEvent) (domain_gen.TweetsResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeGetTweets
	event.Timestamp = time.Now()
	if err := event.Data.FromGetAllTweetsEvent(t); err != nil {
		return domain_gen.TweetsResponse{}, err
	}

	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.TweetsResponse{}, err
	}
	var tweetResp domain_gen.TweetsResponse
	err = json.JSON.Unmarshal(resp, &tweetResp)
	return tweetResp, err
}

func (c *NodeClient) GetUser(host string, e domain_gen.GetUserEvent) (domain_gen.User, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeGetUser
	event.Timestamp = time.Now()
	if err := event.Data.FromGetUserEvent(e); err != nil {
		return domain_gen.User{}, err
	}
	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.User{}, err
	}
	var user domain_gen.User
	err = json.JSON.Unmarshal(resp, &user)
	return user, err
}

func (c *NodeClient) BroadcastNewUser(host string, u domain_gen.NewUserEvent) (domain_gen.User, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeNewUser
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUserEvent(u); err != nil {
		return domain_gen.User{}, err
	}
	resp, err := c.sendEvent(host, event)
	if err != nil {
		return domain_gen.User{}, err
	}
	var user domain_gen.User
	err = json.JSON.Unmarshal(resp, &user)
	return user, err
}

func (c *NodeClient) SendError(host string, e domain_gen.ErrorEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeError
	event.Timestamp = time.Now()
	if err := event.Data.FromErrorEvent(e); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *NodeClient) BroadcastNewFollow(host string, f domain_gen.NewFollowEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeFollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewFollowEvent(f); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *NodeClient) BroadcastNewUnfollow(host string, uf domain_gen.NewUnfollowEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeUnfollow
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUnfollowEvent(uf); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *NodeClient) SendLogin(host string, l domain_gen.LoginEvent) (domain_gen.User, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
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

func (c *NodeClient) SendLogout(host string, l domain_gen.LogoutEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.EventType = domain_gen.EventEventTypeLogout
	event.Timestamp = time.Now()
	if err := event.Data.FromLogoutEvent(l); err != nil {
		return err
	}

	_, err := c.sendEvent(host, event)
	return err
}

func (c *NodeClient) sendEvent(host string, event domain_gen.Event) ([]byte, error) {
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

func (c *NodeClient) GetOwnIPAddress() (string, error) {
	for _, addr := range IPProviders {
		resp, err := http.Get(addr)
		if err != nil {
			continue
		}

		bt, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		return string(bt), nil
	}
	return "", errors.New("no IP address found")
}
