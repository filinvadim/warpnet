package node_client

import (
	"context"
	node_gen "github.com/filinvadim/warpnet/node-gen"
)

const (
	Ping             = node_gen.Ping
	NewTweet         = node_gen.NewTweet
	NewReply         = node_gen.NewReply
	GetReplies       = node_gen.GetReplies
	GetReply         = node_gen.GetReply
	GetTweet         = node_gen.GetTweet
	GetTimeline      = node_gen.GetTimeline
	GetTweets        = node_gen.GetTweets
	GetUser          = node_gen.GetUser
	GetUsers         = node_gen.GetUsers
	NewUser          = node_gen.NewUser
	NewSettingsHosts = node_gen.NewSettingsHosts
	Error            = node_gen.Error
	Follow           = node_gen.Follow
	Unfollow         = node_gen.Unfollow
	Login            = node_gen.Login
	Logout           = node_gen.Logout
)

type (
	NewEventParamsEventType = node_gen.NewEventParamsEventType
	ClientWithResponses     = node_gen.ClientWithResponses
	ClientOption            = node_gen.ClientOption
	Client                  = node_gen.Client
	NewEventJSONRequestBody = node_gen.NewEventJSONRequestBody
	RequestEditorFn         = node_gen.RequestEditorFn
	NewEventResponse        = node_gen.NewEventResponse
)

type NodeRequestor interface {
	NewEventWithResponse(
		ctx context.Context,
		eventType NewEventParamsEventType,
		body NewEventJSONRequestBody,
		reqEditors ...RequestEditorFn,
	) (*NewEventResponse, error)
}

var NewClientWithResponses = node_gen.NewClientWithResponses

func getBody(resp *NewEventResponse) []byte {
	return resp.Body
}
