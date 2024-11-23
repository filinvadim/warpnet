package server

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/config"
	"github.com/filinvadim/dWighter/database"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/json"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type HandlerNodeCacher interface {
	AddNode(n domain_gen.Node)
	GetNodes() []domain_gen.Node
	RemoveNode(n *domain_gen.Node)
}

type NodeRequester interface {
	Ping(host string, ping domain_gen.PingEvent) error
	Pong(host string, ping domain_gen.PongEvent) error
	BroadcastNewTweet(host string, t domain_gen.NewTweetEvent) (domain_gen.Tweet, error)
	BroadcastNewUser(host string, u domain_gen.NewUserEvent) (domain_gen.User, error)
	SendError(host string, e domain_gen.ErrorEvent) error
	BroadcastNewFollow(host string, f domain_gen.NewFollowEvent) error
	BroadcastNewUnfollow(host string, uf domain_gen.NewUnfollowEvent) error
}

type nodeEventHandler struct {
	ownIP        string
	nodeRepo     *database.NodeRepo
	authRepo     *database.AuthRepo
	userRepo     *database.UserRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
	followRepo   *database.FollowRepo
	cli          NodeRequester
	cache        HandlerNodeCacher
	interrupt    chan os.Signal
}

func NewNodeHandler(
	ownIP string,
	cache HandlerNodeCacher,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	timelineRepo *database.TimelineRepo,
	followRepo *database.FollowRepo,
	cli NodeRequester,
	interrupt chan os.Signal,
) (*nodeEventHandler, error) {

	return &nodeEventHandler{
		ownIP:        ownIP,
		nodeRepo:     nodeRepo,
		authRepo:     authRepo,
		userRepo:     userRepo,
		tweetRepo:    tweetRepo,
		timelineRepo: timelineRepo,
		followRepo:   followRepo,
		cli:          cli,
		cache:        cache,
		interrupt:    interrupt,
	}, nil
}

func (d *nodeEventHandler) NewEvent(ctx echo.Context, eventType node_gen.NewEventParamsEventType) (err error) {
	fmt.Println("RECEIVED EVENT: ", eventType)
	var receivedEvent domain_gen.Event
	if err := ctx.Bind(&receivedEvent); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())

	}
	if receivedEvent.Data == nil {
		return nil
	}
	callerHost := ctx.Request().Host
	var response any

	switch eventType {
	case node_gen.Ping:
		if err := d.handlePing(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle ping event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		owner, _ := d.authRepo.Owner()
		u, _ := d.userRepo.Get(owner)
		response = domain_gen.PongEvent{
			CachedNodes: d.cache.GetNodes(),
			DestHost:    &callerHost,
			OwnerInfo:   u,
			OwnerNode:   d.nodeRepo.OwnNode(),
		}
	case node_gen.NewUser:
		userEvent, err := d.handleNewUser(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle new user event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		for _, n := range d.cache.GetNodes() {
			if _, err := d.cli.BroadcastNewUser(n.Host, userEvent); err != nil {
				return err
			}
		}
		response = userEvent.User
	case node_gen.NewTweet:
		tweetEvent, err := d.handleNewTweet(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle new tweet event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		for _, n := range d.cache.GetNodes() {
			fmt.Println("BROADCAST NEW TWEET: ", n.Host)
			if _, err = d.cli.BroadcastNewTweet(n.Host, tweetEvent); err != nil {
				return err
			}
		}
		response = tweetEvent.Tweet
	case node_gen.Follow, node_gen.Unfollow:
		// TODO
	case node_gen.Login:
		if response, err = d.handleLogin(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle login event failure: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case node_gen.Logout:
		if err := d.handleLogout(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle logout event failure: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case node_gen.GetTimeline:
		response, err = d.handleGetTimeline(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle timeline event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case node_gen.GetTweets:
		response, err = d.handleGetTweets(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("get tweets event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case node_gen.GetTweet:
		response, err = d.handleGetSingleTweet(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle single tweet event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case node_gen.GetUser:
		response, err = d.handleGetUser(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle get user event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case node_gen.GetUsers:
		response, err = d.handleGetAllUsers(ctx)
		if err != nil {
			fmt.Printf("handle get all users event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case node_gen.Error:
		if receivedEvent.Data == nil {
			return nil
		}
		errEvent, err := receivedEvent.Data.AsErrorEvent()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		fmt.Printf("received event: %d %s", errEvent.Code, errEvent.Message)
		response = errEvent
	default:
		log.Fatal("UNKNOWN EVENT!!!", eventType)
	}

	bt, _ := json.JSON.Marshal(response)

	fmt.Println("EVENT RESPONSE SUCCESS: ", eventType, string(bt))

	return ctx.JSON(http.StatusOK, response)
}

func (d *nodeEventHandler) handlePing(ctx echo.Context, data *domain_gen.Event_Data) error {
	if ctx.Request().Context().Err() != nil {
		return ctx.Request().Context().Err()
	}
	pingEvent, err := data.AsPingEvent()
	if err != nil {
		return err
	}

	if strings.Contains(ctx.Request().Host, "localhost") {
		return nil
	}

	destHost := ctx.Request().Host
	if pingEvent.OwnerNode.Host == "" {
		pingEvent.OwnerNode.Host = destHost
	}
	if pingEvent.OwnerNode == nil {
		return nil
	}
	d.cache.AddNode(*pingEvent.OwnerNode)

	_, err = d.userRepo.Create(*pingEvent.OwnerInfo)
	if err != nil {
		return err
	}
	_, err = d.nodeRepo.Create(pingEvent.OwnerNode)
	if err != nil {
		return err
	}

	for _, n := range pingEvent.CachedNodes {
		d.cache.AddNode(n)

		_, err = d.nodeRepo.Create(&n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *nodeEventHandler) handleNewUser(ctx echo.Context, data *domain_gen.Event_Data) (domain_gen.NewUserEvent, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.NewUserEvent{}, ctx.Request().Context().Err()
	}
	userEvent, err := data.AsNewUserEvent()
	if err != nil {
		return domain_gen.NewUserEvent{}, err
	}
	if userEvent.User == nil {
		return domain_gen.NewUserEvent{}, nil
	}
	_, err = d.userRepo.Create(*userEvent.User)
	return userEvent, err
}

func (d *nodeEventHandler) handleNewTweet(ctx echo.Context, data *domain_gen.Event_Data) (domain_gen.NewTweetEvent, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.NewTweetEvent{}, ctx.Request().Context().Err()
	}
	tweetEvent, err := data.AsNewTweetEvent()
	if err != nil {
		return domain_gen.NewTweetEvent{}, err
	}
	if tweetEvent.Tweet == nil {
		return domain_gen.NewTweetEvent{}, nil
	}
	t, err := d.tweetRepo.Create(tweetEvent.Tweet.UserId, tweetEvent.Tweet)
	if err != nil {
		return domain_gen.NewTweetEvent{}, err
	}
	owner, err := d.authRepo.Owner()
	if err != nil || owner == "" {
		return domain_gen.NewTweetEvent{}, fmt.Errorf("failed to add tweet to owner timeline: %w", err)
	}

	err = d.timelineRepo.AddTweetToTimeline(owner, *t)
	return domain_gen.NewTweetEvent{}, err
}

func (d *nodeEventHandler) handleLogin(
	ctx echo.Context, data *domain_gen.Event_Data,
) (domain_gen.LoginResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.LoginResponse{}, ctx.Request().Context().Err()
	}
	login, err := data.AsLoginEvent()
	if err != nil {
		return domain_gen.LoginResponse{}, err
	}

	token, err := d.authRepo.Authenticate(login.Username, login.Password)
	if err != nil {
		return domain_gen.LoginResponse{}, err
	}

	ownerId, err := d.authRepo.Owner()
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return domain_gen.LoginResponse{}, err
	}
	owner, err := d.userRepo.Get(ownerId)
	if owner != nil && owner.Username != login.Username {
		return domain_gen.LoginResponse{}, err
	}
	if owner != nil && owner.Username == login.Username {
		return domain_gen.LoginResponse{token, *owner}, nil
	}
	newOwnerId, err := d.authRepo.NewOwner()
	if err != nil {
		return domain_gen.LoginResponse{}, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	now := time.Now()
	u, err := d.userRepo.Create(domain_gen.User{
		CreatedAt: &now,
		UserId:    &newOwnerId,
		Username:  login.Username,
	})
	if err != nil {
		return domain_gen.LoginResponse{}, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_, err = d.nodeRepo.GetByUserId(*u.UserId)
	if err == nil {
		return domain_gen.LoginResponse{token, *u}, nil
	}
	if !errors.Is(err, database.ErrNodeNotFound) {
		fmt.Println("failed to get node:", err)
		return domain_gen.LoginResponse{}, err
	}

	id, err := d.nodeRepo.Create(&domain_gen.Node{
		CreatedAt: &now,
		Host:      d.ownIP + config.InternalNodeAddress.Port(),
		IsActive:  true,
		LastSeen:  now,
		Latency:   nil,
		OwnerId:   *u.UserId,
		Uptime:    func(i int64) *int64 { return &i }(0),
		IsOwned:   true,
	})
	if err != nil {
		return domain_gen.LoginResponse{}, err
	}

	u.NodeId = id
	u, err = d.userRepo.Create(*u)
	if err != nil {
		return domain_gen.LoginResponse{}, err
	}
	return domain_gen.LoginResponse{token, *u}, nil
}

func (d *nodeEventHandler) handleLogout(ctx echo.Context, _ *domain_gen.Event_Data) error {
	if ctx.Request().Context().Err() != nil {
		return ctx.Request().Context().Err()
	}
	d.interrupt <- os.Interrupt
	return nil
}

type TweetsResponse = domain_gen.TweetsResponse

func (d *nodeEventHandler) handleGetTimeline(ctx echo.Context, data *domain_gen.Event_Data) (TweetsResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return TweetsResponse{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTimelineEvent()
	if err != nil {
		return TweetsResponse{}, err
	}
	tweets, nextCursor, err := d.timelineRepo.GetTimeline(event.UserId, event.Limit, event.Cursor)
	if err != nil {
		return TweetsResponse{}, err
	}

	response := TweetsResponse{
		Tweets: tweets,
		Cursor: nextCursor,
	}
	return response, nil
}

func (d *nodeEventHandler) handleGetTweets(ctx echo.Context, data *domain_gen.Event_Data) (TweetsResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return TweetsResponse{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetAllTweetsEvent()
	if err != nil {
		return TweetsResponse{}, err
	}
	tweets, nextCursor, err := d.tweetRepo.List(event.UserId, event.Limit, event.Cursor)
	if err != nil {
		return TweetsResponse{}, err
	}

	response := TweetsResponse{
		Tweets: tweets,
		Cursor: nextCursor,
	}
	return response, nil
}

func (d *nodeEventHandler) handleGetSingleTweet(ctx echo.Context, data *domain_gen.Event_Data) (domain_gen.Tweet, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.Tweet{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTweetEvent()
	if err != nil {
		return domain_gen.Tweet{}, err
	}
	tweet, err := d.tweetRepo.Get(event.UserId, event.TweetId)
	if err != nil {
		return domain_gen.Tweet{}, err
	}
	return *tweet, nil
}

func (d *nodeEventHandler) handleGetUser(ctx echo.Context, data *domain_gen.Event_Data) (domain_gen.User, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.User{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetUserEvent()
	if err != nil {
		return domain_gen.User{}, err
	}
	u, err := d.userRepo.Get(event.UserId)
	if err != nil {
		return domain_gen.User{}, err
	}
	return *u, nil
}

func (d *nodeEventHandler) handleGetAllUsers(ctx echo.Context) (domain_gen.UsersResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return domain_gen.UsersResponse{}, ctx.Request().Context().Err()
	}
	users, err := d.userRepo.List()
	if err != nil {
		return domain_gen.UsersResponse{}, err
	}
	return domain_gen.UsersResponse{"", users}, nil
}
