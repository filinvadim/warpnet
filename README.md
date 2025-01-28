# warpnet

[embedmd]:#(core/stream/routes.go go /const/ /END/)
```go
const ( // START
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
```
