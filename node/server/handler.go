package server

import (
	"errors"
	"fmt"
	"github.com/filinvadim/dWighter/config"
	"github.com/filinvadim/dWighter/database"
	domainGen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/json"
	nodeGen "github.com/filinvadim/dWighter/node-gen"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type NodeRequester interface {
	Ping(host string, ping domainGen.PingEvent) error
	Pong(host string, ping domainGen.PongEvent) error
	BroadcastNewTweet(host string, t domainGen.NewTweetEvent) (domainGen.Tweet, error)
	BroadcastNewUser(host string, u domainGen.NewUserEvent) (domainGen.User, error)
	SendError(host string, e domainGen.ErrorEvent) error
	BroadcastNewFollow(host string, f domainGen.NewFollowEvent) error
	BroadcastNewUnfollow(host string, uf domainGen.NewUnfollowEvent) error
}

type nodeEventHandler struct {
	ownIP        string
	nodeRepo     *database.NodeRepo
	authRepo     *database.AuthRepo
	userRepo     *database.UserRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
	followRepo   *database.FollowRepo
	replyRepo    *database.RepliesRepo
	cli          NodeRequester
	interrupt    chan os.Signal
}

func NewNodeHandler(
	ownIP string,
	nodeRepo *database.NodeRepo,
	authRepo *database.AuthRepo,
	userRepo *database.UserRepo,
	tweetRepo *database.TweetRepo,
	timelineRepo *database.TimelineRepo,
	followRepo *database.FollowRepo,
	replyRepo *database.RepliesRepo,
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
		replyRepo:    replyRepo,
		cli:          cli,
		interrupt:    interrupt,
	}, nil
}

func (d *nodeEventHandler) NewEvent(ctx echo.Context, eventType nodeGen.NewEventParamsEventType) (err error) {
	fmt.Println("RECEIVED EVENT: ", eventType)

	var receivedEvent domainGen.Event
	if err := ctx.Bind(&receivedEvent); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())

	}
	if receivedEvent.Data == nil {
		return nil
	}

	var response any

	switch eventType {
	case nodeGen.Login:
		if response, err = d.handleLogin(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle login event failure: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case nodeGen.Ping:
		if err := d.handlePing(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle ping event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		owner, err := d.authRepo.Owner()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		u, err := d.userRepo.Get(owner)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		nodes, _, err := d.nodeRepo.List(nil, nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		dst := ctx.Request().RemoteAddr
		response = domainGen.PongEvent{
			Nodes:     nodes,
			DestHost:  &dst,
			OwnerInfo: &u,
			OwnerNode: d.nodeRepo.OwnNode(),
		}
	case nodeGen.NewUser:
		userEvent, err := d.handleNewUser(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle new user event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		nodes, _, err := d.nodeRepo.List(nil, nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		for _, n := range nodes {
			if n.IsOwned {
				continue
			}
			if _, err := d.cli.BroadcastNewUser(n.Host, userEvent); err != nil {
				return err
			}
		}
		response = userEvent.User
	case nodeGen.NewTweet:
		tweet, err := d.handleNewTweet(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle new tweet event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		nodes, _, err := d.nodeRepo.List(nil, nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		for _, n := range nodes {
			if n.IsOwned {
				continue
			}
			if _, err = d.cli.BroadcastNewTweet(n.Host, domainGen.NewTweetEvent{&tweet}); err != nil {
				return err
			}
		}
		response = tweet
	case nodeGen.NewReply:
		response, err = d.handleNewReply(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle new reply event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetReplies:
	case nodeGen.Follow, nodeGen.Unfollow:
		// TODO

	case nodeGen.Logout:
		if err := d.handleLogout(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle logout event failure: %v", err)
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case nodeGen.GetTimeline:
		response, err = d.handleGetTimeline(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle timeline event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetTweets:
		response, err = d.handleGetTweets(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("get tweets event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetTweet:
		response, err = d.handleGetSingleTweet(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle single tweet event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetUser:
		response, err = d.handleGetUser(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle get user event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetUsers:
		response, err = d.handleGetAllUsers(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle get all users event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.NewSettingsHosts:
		if err = d.handleNewSettingHosts(ctx, receivedEvent.Data); err != nil {
			fmt.Printf("handle new settings hosts event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.GetSettingsHosts:
		response, err = d.handleGetSettingHosts(ctx, receivedEvent.Data)
		if err != nil {
			fmt.Printf("handle get settings hosts event failure: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case nodeGen.Error:
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

	{
		bt, _ := json.JSON.Marshal(response)
		fmt.Println("EVENT RESPONSE SUCCESS: ", eventType, string(bt))
	}

	return ctx.JSON(http.StatusOK, response)
}

func (d *nodeEventHandler) handleLogin(
	ctx echo.Context, data *domainGen.Event_Data,
) (domainGen.LoginResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.LoginResponse{}, ctx.Request().Context().Err()
	}
	login, err := data.AsLoginEvent()
	if err != nil {
		return domainGen.LoginResponse{}, err
	}

	token, err := d.authRepo.Authenticate(login.Username, login.Password)
	if err != nil {
		return domainGen.LoginResponse{}, err
	}

	fmt.Println(token, "TOKEN")
	ownerId, err := d.authRepo.Owner()
	if err != nil && !errors.Is(err, database.ErrOwnerNotFound) {
		return domainGen.LoginResponse{}, err
	}
	if ownerId == "" {
		fmt.Println("OWNER ID is empty")

		ownerId, err = d.authRepo.NewOwner()
		if err != nil {
			return domainGen.LoginResponse{}, fmt.Errorf("new owner: %w", err)
		}
	}

	fmt.Println(ownerId, "OWNER ID ????? ")

	owner, err := d.userRepo.Get(ownerId)
	fmt.Println(err, "USER GET EERRR")
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		return domainGen.LoginResponse{}, err
	}
	fmt.Println(owner, "OWNER STRUCT")

	if owner.Username != login.Username {
		return domainGen.LoginResponse{}, fmt.Errorf("user %s doesn't exist", login.Username)
	}
	if owner.Username == login.Username {
		return domainGen.LoginResponse{token, owner}, nil
	}

	var (
		now    = time.Now()
		nodeId = uuid.New()
	)
	u, err := d.userRepo.Create(domainGen.User{
		CreatedAt: now,
		Id:        ownerId,
		Username:  login.Username,
		NodeId:    nodeId,
	})
	if err != nil {
		return domainGen.LoginResponse{}, fmt.Errorf("create user: %w", err)
	}
	fmt.Println("USER CREATED", ownerId)

	_, err = d.nodeRepo.GetByUserId(ownerId)
	if err == nil {
		return domainGen.LoginResponse{token, u}, nil
	}
	if !errors.Is(err, database.ErrNodeNotFound) {
		fmt.Println("failed to get node:", err)
		return domainGen.LoginResponse{}, fmt.Errorf("get node: %w", err)
	}

	_, err = d.nodeRepo.Create(domainGen.Node{
		Id:        nodeId,
		CreatedAt: now,
		Host:      d.ownIP + ":" + config.InternalNodeAddress.Port(),
		IsActive:  true,
		LastSeen:  now,
		OwnerId:   ownerId,
		Uptime:    func(i int64) *int64 { return &i }(0),
		IsOwned:   true,
	})
	if err != nil {
		return domainGen.LoginResponse{}, fmt.Errorf("create owner's node: %w", err)
	}
	return domainGen.LoginResponse{token, u}, nil
}

func (d *nodeEventHandler) handlePing(ctx echo.Context, data *domainGen.Event_Data) error {
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
	if pingEvent.OwnerNode == nil {
		return nil
	}

	destHost := ctx.Request().Host
	if pingEvent.OwnerNode.Host == "" {
		pingEvent.OwnerNode.Host = destHost
	}

	_, err = d.userRepo.Create(*pingEvent.OwnerInfo)
	if err != nil {
		return err
	}
	_, err = d.nodeRepo.Create(*pingEvent.OwnerNode)
	if err != nil {
		return err
	}

	for _, n := range pingEvent.Nodes {
		_, err = d.nodeRepo.Create(n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *nodeEventHandler) handleNewSettingHosts(ctx echo.Context, data *domainGen.Event_Data) error {
	if ctx.Request().Context().Err() != nil {
		return ctx.Request().Context().Err()
	}
	hostsEvent, err := data.AsNewSettingsHostsEvent()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, h := range hostsEvent.Hosts {
		_, err := d.nodeRepo.Create(domainGen.Node{
			CreatedAt: now,
			Host:      h,
			IsActive:  true,
			IsOwned:   false,
			LastSeen:  now,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *nodeEventHandler) handleGetSettingHosts(ctx echo.Context, data *domainGen.Event_Data) (domainGen.HostsResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.HostsResponse{}, ctx.Request().Context().Err()
	}
	hostsEvent, err := data.AsGetSettingsHostsEvent()
	if err != nil {
		return domainGen.HostsResponse{}, err
	}

	nodes, cursor, err := d.nodeRepo.List(hostsEvent.Limit, hostsEvent.Cursor)
	if err != nil {
		return domainGen.HostsResponse{}, err
	}

	hosts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		hosts = append(hosts, n.Host)
	}
	return domainGen.HostsResponse{Cursor: cursor, Hosts: hosts}, nil
}

func (d *nodeEventHandler) handleNewUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.NewUserEvent, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.NewUserEvent{}, ctx.Request().Context().Err()
	}
	userEvent, err := data.AsNewUserEvent()
	if err != nil {
		return domainGen.NewUserEvent{}, err
	}
	if userEvent.User == nil {
		return domainGen.NewUserEvent{}, nil
	}
	_, err = d.userRepo.Create(*userEvent.User)
	return userEvent, err
}

func (d *nodeEventHandler) handleNewReply(ctx echo.Context, data *domainGen.Event_Data) (reply domainGen.Tweet, err error) {
	if ctx.Request().Context().Err() != nil {
		return reply, ctx.Request().Context().Err()
	}
	replyEvent, err := data.AsNewReplyEvent()
	if err != nil {
		return reply, err
	}
	if replyEvent.Tweet == nil {
		return reply, nil
	}
	r, err := d.replyRepo.AddReply(*replyEvent.Tweet)
	if err != nil {
		return reply, err
	}
	return r, err
}

type RepliesResponse = domainGen.RepliesTreeResponse

func (d *nodeEventHandler) handleGetReplies(ctx echo.Context, data *domainGen.Event_Data) (response RepliesResponse, err error) {
	if ctx.Request().Context().Err() != nil {
		return response, ctx.Request().Context().Err()
	}
	event, err := data.AsGetRepliesEvent()
	if err != nil {
		return response, err
	}
	replies, nextCursor, err := d.replyRepo.GetRepliesTree(
		event.RootId, event.ParentReplyId, event.Limit, event.Cursor,
	)
	if err != nil {
		return response, err
	}

	response = RepliesResponse{
		Replies: replies,
		Cursor:  nextCursor,
	}
	return response, nil
}

func (d *nodeEventHandler) handleNewTweet(ctx echo.Context, data *domainGen.Event_Data) (t domainGen.Tweet, err error) {
	if ctx.Request().Context().Err() != nil {
		return t, ctx.Request().Context().Err()
	}
	tweetEvent, err := data.AsNewTweetEvent()
	if err != nil {
		return t, err
	}
	if tweetEvent.Tweet == nil {
		return t, nil
	}
	t, err = d.tweetRepo.Create(tweetEvent.Tweet.UserId, *tweetEvent.Tweet)
	if err != nil {
		return t, err
	}
	owner, err := d.authRepo.Owner()
	if err != nil || owner == "" {
		return t, fmt.Errorf("failed to add tweet to owner timeline: %w", err)
	}

	err = d.timelineRepo.AddTweetToTimeline(owner, t)
	return t, err
}

func (d *nodeEventHandler) handleLogout(ctx echo.Context, _ *domainGen.Event_Data) error {
	if ctx.Request().Context().Err() != nil {
		return ctx.Request().Context().Err()
	}
	d.interrupt <- os.Interrupt
	return nil
}

type TweetsResponse = domainGen.TweetsResponse

func (d *nodeEventHandler) handleGetTimeline(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return TweetsResponse{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTimelineEvent()
	if err != nil {
		return TweetsResponse{}, err
	}
	if _, err := d.userRepo.Get(event.UserId); err != nil && errors.Is(err, database.ErrUserNotFound) {
		return TweetsResponse{Tweets: []domainGen.Tweet{}, Cursor: ""}, nil
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

func (d *nodeEventHandler) handleGetTweets(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
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

func (d *nodeEventHandler) handleGetSingleTweet(ctx echo.Context, data *domainGen.Event_Data) (domainGen.Tweet, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.Tweet{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTweetEvent()
	if err != nil {
		return domainGen.Tweet{}, err
	}
	tweet, err := d.tweetRepo.Get(event.UserId, event.TweetId)
	if err != nil {
		return domainGen.Tweet{}, err
	}
	return tweet, nil
}

func (d *nodeEventHandler) handleGetUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.User, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.User{}, ctx.Request().Context().Err()
	}

	event, err := data.AsGetUserEvent()
	if err != nil {
		return domainGen.User{}, err
	}

	u, err := d.userRepo.Get(event.UserId)
	if err != nil {
		return domainGen.User{}, err
	}
	return u, nil
}

func (d *nodeEventHandler) handleGetAllUsers(ctx echo.Context, data *domainGen.Event_Data) (domainGen.UsersResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.UsersResponse{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetAllUsersEvent()
	if err != nil {
		return domainGen.UsersResponse{}, err
	}

	users, cur, err := d.userRepo.List(event.Limit, event.Cursor)
	if err != nil {
		return domainGen.UsersResponse{}, err
	}

	return domainGen.UsersResponse{cur, users}, nil
}
