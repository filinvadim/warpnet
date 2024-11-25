package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/dWighter/config"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/json"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"io"
	"net/http"
	"strings"
	"time"
)

type NodeClient struct {
	ctx context.Context
	cli *node_gen.ClientWithResponses
}

func NewNodeClient(ctx context.Context) (*NodeClient, error) {
	cli, err := node_gen.NewClientWithResponses(config.InternalNodeAddress.String(), func(client *node_gen.Client) error {
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &NodeClient{ctx, cli}, nil
}

func (c *NodeClient) Ping(host string, ping domain_gen.PingEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromPingEvent(ping); err != nil {
		return err
	}
	_, err := c.sendEvent(host, node_gen.Ping, event)
	return err
}
func (c *NodeClient) Pong(host string, ping domain_gen.PongEvent) error {

	return nil
}

func (c *NodeClient) BroadcastNewTweet(host string, t domain_gen.NewTweetEvent) (domain_gen.Tweet, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromNewTweetEvent(t); err != nil {
		return domain_gen.Tweet{}, err
	}

	resp, err := c.sendEvent(host, node_gen.NewTweet, event)
	if err != nil {
		return domain_gen.Tweet{}, err
	}
	var tweet domain_gen.Tweet
	err = json.JSON.Unmarshal(resp, &tweet)
	return tweet, err
}

func (c *NodeClient) SendGetTweet(host string, t domain_gen.GetTweetEvent) (domain_gen.Tweet, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromGetTweetEvent(t); err != nil {
		return domain_gen.Tweet{}, err
	}

	resp, err := c.sendEvent(host, node_gen.GetTweet, event)
	if err != nil {
		return domain_gen.Tweet{}, err
	}
	var tweet domain_gen.Tweet
	err = json.JSON.Unmarshal(resp, &tweet)
	return tweet, err
}

func (c *NodeClient) SendGetTimeline(host string, t domain_gen.GetTimelineEvent) (domain_gen.TweetsResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromGetTimelineEvent(t); err != nil {
		return domain_gen.TweetsResponse{}, err
	}

	resp, err := c.sendEvent(host, node_gen.GetTimeline, event)
	if err != nil {
		return domain_gen.TweetsResponse{}, err
	}
	var tweetResp domain_gen.TweetsResponse
	err = json.JSON.Unmarshal(resp, &tweetResp)
	return tweetResp, err
}

func (c *NodeClient) SendGetAllTweets(host string, t domain_gen.GetAllTweetsEvent) (domain_gen.TweetsResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromGetAllTweetsEvent(t); err != nil {
		return domain_gen.TweetsResponse{}, err
	}

	resp, err := c.sendEvent(host, node_gen.GetTweets, event)
	if err != nil {
		return domain_gen.TweetsResponse{}, err
	}
	var tweetResp domain_gen.TweetsResponse
	err = json.JSON.Unmarshal(resp, &tweetResp)
	return tweetResp, err
}

func (c *NodeClient) GetUser(host string, e domain_gen.GetUserEvent) (domain_gen.User, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromGetUserEvent(e); err != nil {
		return domain_gen.User{}, err
	}
	resp, err := c.sendEvent(host, node_gen.GetUser, event)
	if err != nil {
		return domain_gen.User{}, err
	}
	var user domain_gen.User
	err = json.JSON.Unmarshal(resp, &user)
	return user, err
}

func (c *NodeClient) GetUsers(host string, e domain_gen.GetAllUsersEvent) (domain_gen.UsersResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromGetAllUsersEvent(e); err != nil {
		return domain_gen.UsersResponse{}, err
	}
	resp, err := c.sendEvent(host, node_gen.GetUsers, event)
	if err != nil {
		return domain_gen.UsersResponse{}, err
	}
	var usersResp domain_gen.UsersResponse
	err = json.JSON.Unmarshal(resp, &usersResp)
	return usersResp, err
}

func (c *NodeClient) BroadcastNewUser(host string, u domain_gen.NewUserEvent) (domain_gen.User, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUserEvent(u); err != nil {
		return domain_gen.User{}, err
	}
	resp, err := c.sendEvent(host, node_gen.NewUser, event)
	if err != nil {
		return domain_gen.User{}, err
	}
	var user domain_gen.User
	err = json.JSON.Unmarshal(resp, &user)
	return user, err
}

func (c *NodeClient) SendError(host string, e domain_gen.ErrorEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromErrorEvent(e); err != nil {
		return err
	}

	_, err := c.sendEvent(host, node_gen.Error, event)
	return err
}

func (c *NodeClient) BroadcastNewFollow(host string, f domain_gen.NewFollowEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromNewFollowEvent(f); err != nil {
		return err
	}

	_, err := c.sendEvent(host, node_gen.Follow, event)
	return err
}

func (c *NodeClient) BroadcastNewUnfollow(host string, uf domain_gen.NewUnfollowEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromNewUnfollowEvent(uf); err != nil {
		return err
	}

	_, err := c.sendEvent(host, node_gen.Unfollow, event)
	return err
}

func (c *NodeClient) SendLogin(host string, l domain_gen.LoginEvent) (domain_gen.LoginResponse, error) {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromLoginEvent(l); err != nil {
		return domain_gen.LoginResponse{}, err
	}

	resp, err := c.sendEvent(host, node_gen.Login, event)
	if err != nil {
		return domain_gen.LoginResponse{}, err
	}
	var loginResp domain_gen.LoginResponse
	err = json.JSON.Unmarshal(resp, &loginResp)
	return loginResp, err
}

func (c *NodeClient) SendLogout(host string, l domain_gen.LogoutEvent) error {
	event := domain_gen.Event{Data: &domain_gen.Event_Data{}}
	event.Timestamp = time.Now()
	if err := event.Data.FromLogoutEvent(l); err != nil {
		return err
	}

	_, err := c.sendEvent(host, node_gen.Logout, event)
	return err
}

func (c *NodeClient) sendEvent(
	host string, eventType node_gen.NewEventParamsEventType, event domain_gen.Event,
) ([]byte, error) {
	if strings.Contains(host, "localhost") {
		return nil, nil
	}
	resp, err := c.cli.NewEventWithResponse(c.ctx, eventType, event, func(ctx context.Context, req *http.Request) error {
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
	for _, addr := range config.IPProviders {
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
