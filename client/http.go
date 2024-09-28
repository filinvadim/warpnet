package client

import (
	"context"
	"errors"
	"github.com/filinvadim/dWighter/api/client"
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

func (cli *Client) Ping(addr string) (*client.PingResponse, error) {
	req, err := client.NewGetNodesPingRequest(addr)
	if err != nil {
		return nil, err
	}
	resp, err := cli.c.Do(req)
	if err != nil {
		return nil, err
	}
	pingResp, err := client.ParseGetNodesPingResponse(resp)
	if err != nil {
		return nil, err
	}

	if pingResp == nil {
		return nil, errors.New("empty ping response")
	}
	return pingResp.JSON200, pingResp.HTTPResponse.Body.Close()
}
