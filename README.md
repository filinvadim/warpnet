# WARPNET
## General principles
- warpnet must be independent of any 3rd party services
- warpnet must be independent of any 3rd party technologies
- warpnet must be represented as a single executable file
- warpnet must be a multiplatform service
- warpnet must be a singleton node that running on machine, but it could have as many aliases as needed
- warpnet member node must be managed only by nodes consensus
- warpnet business node could have centralized management
- warpnet node must store private data only at host machine

## Requirements
    - golang >=1.23

## How to run
- bootstrap node
```bash 
go run cmd/node/bootstrap/main.go
```
- member node
```bash 
go run cmd/node/member/main.go
```

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
