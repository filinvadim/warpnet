package stream

import (
	"github.com/filinvadim/warpnet/core/warpnet"
)

const (
	PairPrivate     warpnet.WarpRoute = "/private/pair/1.0.0"
	TimelinePrivate warpnet.WarpRoute = "/private/timeline/1.0.0"
	TweetPrivate    warpnet.WarpRoute = "/private/tweet/1.0.0"

	UserPublic   warpnet.WarpRoute = "/public/user/1.0.0"
	TweetsPublic warpnet.WarpRoute = "/public/tweets/1.0.0"
	InfoPublic   warpnet.WarpRoute = "/public/info/1.0.0"
)
