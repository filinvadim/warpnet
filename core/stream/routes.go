package stream

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	"strings"
)

// package api;type WarpRoute string

const (
	LoginPostPrivate  WarpRoute = "/private/post/login/1.0.0"
	LogoutPostPrivate WarpRoute = "/private/post/logout/1.0.0"

	PairPostPrivate    WarpRoute = "/private/post/pair/1.0.0"
	TimelineGetPrivate WarpRoute = "/private/get/timeline/1.0.0"
	TweetPostPrivate   WarpRoute = "/private/post/tweet/1.0.0"
	ReplyPostPrivate   WarpRoute = "/private/post/reply/1.0.0"
	ReplyDeletePrivate WarpRoute = "/private/delete/reply/1.0.0"
	TweetDeletePrivate WarpRoute = "/private/delete/tweet/1.0.0"

	UserGetPublic    WarpRoute = "/public/get/user/1.0.0"
	UsersGetPublic   WarpRoute = "/public/get/users/1.0.0"
	TweetsGetPublic  WarpRoute = "/public/get/tweets/1.0.0"
	TweetGetPublic   WarpRoute = "/public/get/tweet/1.0.0"
	RepliesGetPublic WarpRoute = "/public/get/replies/1.0.0"
	ReplyGetPublic   WarpRoute = "/public/get/reply/1.0.0"
	InfoGetPublic    WarpRoute = "/public/get/info/1.0.0"
) // END

type WarpRoute string

func (r WarpRoute) ProtocolID() protocol.ID {
	return protocol.ID(r)
}

func (r WarpRoute) String() string {
	return string(r)
}

func (r WarpRoute) IsPrivate() bool {
	return strings.Contains(string(r), "private")
}

type WarpRoutes []WarpRoute

func (rs WarpRoutes) FromRoutesToPrIDs() []protocol.ID {
	prIDs := make([]protocol.ID, 0, len(rs))
	for _, r := range rs {
		prIDs = append(prIDs, r.ProtocolID())
	}
	return prIDs
}

func FromPrIDToRoute(prID protocol.ID) WarpRoute {
	return WarpRoute(prID)
}

func FromPrIDToRoutes(prIDs []protocol.ID) WarpRoutes {
	rs := make(WarpRoutes, 0, len(prIDs))
	for _, p := range prIDs {
		rs = append(rs, WarpRoute(p))
	}
	return rs
}
