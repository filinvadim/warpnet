package discovery

import (
	"context"
	"errors"
	"github.com/filinvadim/dWighter/api/discovery"
	cr "github.com/filinvadim/dWighter/crypto"
	"net/http"
	"time"
)

type ClientLogger interface {
}

type Client struct {
	ctx context.Context
	c   *http.Client
	l   ClientLogger
}

func New(ctx context.Context, nodeID string, l ClientLogger) (*Client, error) {
	conf, err := cr.GenerateTLSConfig(nodeID)
	if err != nil {
		return nil, err
	}
	cli := http.DefaultClient
	tr := &http.Transport{
		TLSClientConfig: conf,
	}
	cli.Transport = tr
	cli.Timeout = 10 * time.Second
	return &Client{ctx, cli, l}, nil
}

func (cli *Client) Ping(addr string) (*discovery.Event, error) {
	req, err := discovery.NewNewEventRequest(addr, discovery.Event{}) // TODO
	if err != nil {
		return nil, err
	}
	resp, err := cli.c.Do(req)
	if err != nil {
		return nil, err
	}
	pingResp, err := discovery.ParseNewEventResponse(resp)
	if err != nil {
		return nil, err
	}

	if pingResp == nil {
		return nil, errors.New("empty ping response")
	}
	return pingResp.JSON200, pingResp.HTTPResponse.Body.Close()
}
