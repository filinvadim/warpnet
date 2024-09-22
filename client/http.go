package client

import (
	"context"
	"errors"
	"github.com/filinvadim/dWighter/api/client"
	"net/http"
)

type ClientLogger interface {
}

type Client struct {
	ctx context.Context
	c   *http.Client
	l   ClientLogger
}

func New(ctx context.Context, l ClientLogger) *Client {
	return &Client{ctx, http.DefaultClient, l}
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
