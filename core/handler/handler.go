package handler

//
//import (
//	"errors"
//	"fmt"
//	"github.com/filinvadim/warpnet/database"
//	domainGen "github.com/filinvadim/warpnet/domain-gen"
//	"github.com/filinvadim/warpnet/json"
//	nodeGen "github.com/filinvadim/warpnet/core-gen"
//	"github.com/labstack/echo/v4"
//	"log"
//	"net/http"
//)
//
//type NodeStreamHandler struct {
//}
//
//func NewNodeStreamHandler() (*NodeStreamHandler, error) {
//	return &NodeStreamHandler{}, nil
//}
//
//func (h *NodeStreamHandler) Handle() (err error) {
//
//	var receivedEvent domainGen.Event
//	if err := ctx.Bind(&receivedEvent); err != nil {
//		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
//
//	}
//	if receivedEvent.Data == nil {
//		return nil
//	}
//
//	var response any
//
//	switch eventType {
//	case nodeGen.NewUser:
//		userEvent, err := d.handleNewUser(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle new user event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//		nodes, _, err := d.nodeRepo.List(nil, nil)
//		if err != nil {
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//		for _, n := range nodes {
//			if n.IsOwned {
//				continue
//			}
//			if _, err := d.cli.BroadcastNewUser(n.Host, userEvent); err != nil {
//				return err
//			}
//		}
//		response = userEvent.User
//	case nodeGen.NewTweet:
//		tweet, err := d.handleNewTweet(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle new tweet event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//		nodes, _, err := d.nodeRepo.List(nil, nil)
//		if err != nil {
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//		for _, n := range nodes {
//			if n.IsOwned {
//				continue
//			}
//			if _, err = d.cli.BroadcastNewTweet(n.Host, domainGen.NewTweetEvent{&tweet}); err != nil {
//				return err
//			}
//		}
//		response = tweet
//	case nodeGen.NewReply:
//		response, err = d.handleNewReply(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle new reply event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetReply:
//		response, err = d.handleGetSingleReply(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle get single reply event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetReplies:
//		response, err = d.handleGetReplies(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle get replies event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.Follow, nodeGen.Unfollow:
//		// TODO
//
//	case nodeGen.GetTimeline:
//		response, err = d.handleGetTimeline(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle timeline event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetTweets:
//		response, err = d.handleGetTweets(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("get tweets event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetTweet:
//		response, err = d.handleGetSingleTweet(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle single tweet event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetUser:
//		response, err = d.handleGetUser(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle get user event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.GetUsers:
//		response, err = d.handleGetAllUsers(ctx, receivedEvent.Data)
//		if err != nil {
//			fmt.Printf("handle get all users event failure: %v", err)
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//	case nodeGen.Error:
//		if receivedEvent.Data == nil {
//			return nil
//		}
//		errEvent, err := receivedEvent.Data.AsErrorEvent()
//		if err != nil {
//			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//		}
//
//		fmt.Printf("received event: %d %s", errEvent.Code, errEvent.Message)
//		response = errEvent
//	default:
//		log.Fatal("UNKNOWN EVENT!!!", eventType)
//	}
//
//	{
//		bt, _ := json.JSON.Marshal(response)
//		fmt.Println("EVENT RESPONSE SUCCESS: ", eventType, string(bt))
//	}
//
//	return ctx.JSON(http.StatusOK, response)
//}
//
//func (d *nodeEventHandler) handleNewUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.NewUserEvent, error) {
//	if ctx.Request().Context().Err() != nil {
//		return domainGen.NewUserEvent{}, ctx.Request().Context().Err()
//	}
//	userEvent, err := data.AsNewUserEvent()
//	if err != nil {
//		return domainGen.NewUserEvent{}, err
//	}
//	if userEvent.User == nil {
//		return domainGen.NewUserEvent{}, nil
//	}
//	_, err = d.userRepo.Create(*userEvent.User)
//	return userEvent, err
//}
//
//func (d *nodeEventHandler) handleNewReply(ctx echo.Context, data *domainGen.Event_Data) (reply domainGen.Tweet, err error) {
//	if ctx.Request().Context().Err() != nil {
//		return reply, ctx.Request().Context().Err()
//	}
//	replyEvent, err := data.AsNewReplyEvent()
//	if err != nil {
//		return reply, err
//	}
//	if replyEvent.Tweet == nil {
//		return reply, nil
//	}
//	r, err := d.replyRepo.AddReply(*replyEvent.Tweet)
//	if err != nil {
//		return reply, err
//	}
//	return r, err
//}
//
//type RepliesResponse = domainGen.RepliesTreeResponse
//
//func (d *nodeEventHandler) handleGetReplies(ctx echo.Context, data *domainGen.Event_Data) (response RepliesResponse, err error) {
//	if ctx.Request().Context().Err() != nil {
//		return response, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetRepliesEvent()
//	if err != nil {
//		return response, err
//	}
//	replies, nextCursor, err := d.replyRepo.GetRepliesTree(
//		event.RootId, event.ParentReplyId, event.Limit, event.Cursor,
//	)
//	if err != nil {
//		return response, err
//	}
//
//	response = RepliesResponse{
//		Replies: replies,
//		Cursor:  nextCursor,
//	}
//	return response, nil
//}
//func (d *nodeEventHandler) handleGetSingleReply(ctx echo.Context, data *domainGen.Event_Data) (domainGen.Tweet, error) {
//	if ctx.Request().Context().Err() != nil {
//		return domainGen.Tweet{}, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetReplyEvent()
//	if err != nil {
//		return domainGen.Tweet{}, err
//	}
//	tweet, err := d.replyRepo.GetReply(event.RootId, event.ParentReplyId, event.ReplyId)
//	if err != nil {
//		return domainGen.Tweet{}, err
//	}
//	return tweet, nil
//}
//
//func (d *nodeEventHandler) handleNewTweet(ctx echo.Context, data *domainGen.Event_Data) (t domainGen.Tweet, err error) {
//	if ctx.Request().Context().Err() != nil {
//		return t, ctx.Request().Context().Err()
//	}
//	tweetEvent, err := data.AsNewTweetEvent()
//	if err != nil {
//		return t, err
//	}
//	if tweetEvent.Tweet == nil {
//		return t, nil
//	}
//	t, err = d.tweetRepo.Create(tweetEvent.Tweet.UserId, *tweetEvent.Tweet)
//	if err != nil {
//		return t, err
//	}
//	owner, err := d.authRepo.Owner()
//	if err != nil || owner == "" {
//		return t, fmt.Errorf("failed to add tweet to owner timeline: %w", err)
//	}
//
//	err = d.timelineRepo.AddTweetToTimeline(owner, t)
//	return t, err
//}
//
//type TweetsResponse = domainGen.TweetsResponse
//
//func (d *nodeEventHandler) handleGetTimeline(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
//	if ctx.Request().Context().Err() != nil {
//		return TweetsResponse{}, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetTimelineEvent()
//	if err != nil {
//		return TweetsResponse{}, err
//	}
//	if _, err := d.userRepo.Get(event.UserId); err != nil && errors.Is(err, database.ErrUserNotFound) {
//		return TweetsResponse{Tweets: []domainGen.Tweet{}, Cursor: ""}, nil
//	}
//
//	tweets, nextCursor, err := d.timelineRepo.GetTimeline(event.UserId, event.Limit, event.Cursor)
//	if err != nil {
//		return TweetsResponse{}, err
//	}
//
//	response := TweetsResponse{
//		Tweets: tweets,
//		Cursor: nextCursor,
//	}
//	return response, nil
//}
//
//func (d *nodeEventHandler) handleGetTweets(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
//	if ctx.Request().Context().Err() != nil {
//		return TweetsResponse{}, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetAllTweetsEvent()
//	if err != nil {
//		return TweetsResponse{}, err
//	}
//	tweets, nextCursor, err := d.tweetRepo.List(event.UserId, event.Limit, event.Cursor)
//	if err != nil {
//		return TweetsResponse{}, err
//	}
//
//	response := TweetsResponse{
//		Tweets: tweets,
//		Cursor: nextCursor,
//	}
//	return response, nil
//}
//
//func (d *nodeEventHandler) handleGetSingleTweet(ctx echo.Context, data *domainGen.Event_Data) (domainGen.Tweet, error) {
//	if ctx.Request().Context().Err() != nil {
//		return domainGen.Tweet{}, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetTweetEvent()
//	if err != nil {
//		return domainGen.Tweet{}, err
//	}
//	tweet, err := d.tweetRepo.Get(event.UserId, event.TweetId)
//	if err != nil {
//		return domainGen.Tweet{}, err
//	}
//	return tweet, nil
//}
//
//func (d *nodeEventHandler) handleGetUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.User, error) {
//	if ctx.Request().Context().Err() != nil {
//		return domainGen.User{}, ctx.Request().Context().Err()
//	}
//
//	event, err := data.AsGetUserEvent()
//	if err != nil {
//		return domainGen.User{}, err
//	}
//
//	u, err := d.userRepo.Get(event.UserId)
//	if err != nil {
//		return domainGen.User{}, err
//	}
//	return u, nil
//}
//
//func (d *nodeEventHandler) handleGetAllUsers(ctx echo.Context, data *domainGen.Event_Data) (domainGen.UsersResponse, error) {
//	if ctx.Request().Context().Err() != nil {
//		return domainGen.UsersResponse{}, ctx.Request().Context().Err()
//	}
//	event, err := data.AsGetAllUsersEvent()
//	if err != nil {
//		return domainGen.UsersResponse{}, err
//	}
//
//	users, cur, err := d.userRepo.List(event.Limit, event.Cursor)
//	if err != nil {
//		return domainGen.UsersResponse{}, err
//	}
//
//	return domainGen.UsersResponse{cur, users}, nil
//}
