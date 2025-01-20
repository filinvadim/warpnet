package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/database"
	event "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
)

type StreamRegisterer interface {
	SetStreamHandler(pid types.WarpDiscriminator, handler network.StreamHandler)
}

type StreamLogger interface {
	Info(args ...interface{})
}

type NodeStreamHandler struct {
	ctx          context.Context
	n            StreamRegisterer
	l            StreamLogger
	userRepo     *database.UserRepo
	replyRepo    *database.ReplyRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
}

func NewNodeStreamHandler(
	ctx context.Context,
	n StreamRegisterer,
	l StreamLogger,
	userRepo *database.UserRepo,
	replyRepo *database.ReplyRepo,
	tweetRepo *database.TweetRepo,
	timelineRepo *database.TimelineRepo,
) (*NodeStreamHandler, error) {

	return &NodeStreamHandler{
		ctx:          ctx,
		n:            n,
		l:            l,
		userRepo:     userRepo,
		replyRepo:    replyRepo,
		tweetRepo:    tweetRepo,
		timelineRepo: timelineRepo,
	}, nil
}

func (d *NodeStreamHandler) RegisterHandlers(paths *openapi3.Paths) error {
	for k := range paths.Map() {
		d.l.Info("registering handler: " + k)
		d.n.SetStreamHandler(types.WarpDiscriminator(k), func(s network.Stream) {
			defer s.Close()
			log.Println("new stream opened", types.WarpDiscriminator(k), s.Conn().RemotePeer())

			var (
				response any
				err      error
			)

			buf := bytes.NewBuffer(nil)
			_, err = buf.ReadFrom(s)
			if err != nil {
				log.Printf("fail reading from stream: %s", err)
				return
			}

			var value interface{}

			//log.Printf("Received message: %s", buf.String())
			//var ev event.Event
			//err = json.JSON.Unmarshal(buf.Bytes(), &ev)
			//if err != nil {
			//	log.Printf("fail unmarshaling event: %s", err)
			//	return
			//}
			//value, err := ev.Data.ValueByDiscriminator()
			//if err != nil {
			//	log.Printf("fail getting discriminator value: %s", err)
			//	return
			//}

			switch value.(type) {
			//case event.NewUserEvent:
			//	response, err = d.handleNewUser(d.ctx, ev.Data)
			//case event.GetUserEvent:
			//	response, err = d.handleGetUser(d.ctx, ev.Data)
			//case event.GetAllUsersEvent:
			//	response, err = d.handleGetAllUsers(d.ctx, ev.Data)
			//case event.NewChatEvent:
			//	//response, err = d.handleNewChat(d.ctx, ev.Data)
			//case event.GetChatEvent:
			//	//response, err = d.handleGetChat(d.ctx, ev.Data)
			//case event.GetAllChatsEvent:
			//	//response, err = d.handleGetAllChats(d.ctx, ev.Data)
			//case event.NewTweetEvent:
			//	response, err = d.handleNewTweet(d.ctx, ev.Data)
			//case event.GetTweetEvent:
			//	response, err = d.handleGetTweet(d.ctx, ev.Data)
			//case event.GetAllTweetsEvent:
			//	response, err = d.handleGetAllTweets(d.ctx, ev.Data)
			//case event.GetTimelineEvent:
			//	response, err = d.handleGetTimeline(d.ctx, ev.Data)
			//case event.NewReplyEvent:
			//	response, err = d.handleNewReply(d.ctx, ev.Data)
			//case event.GetReplyEvent:
			//	response, err = d.handleGetReply(d.ctx, ev.Data)
			//case event.GetAllRepliesEvent:
			//	response, err = d.handleGetAllReplies(d.ctx, ev.Data)
			case event.NewMessageEvent:
				//response, err = d.handleNewChat(d.ctx, ev.Data)
			case event.GetMessageEvent:
				//response, err = d.handleNewChat(d.ctx, ev.Data)
			case event.GetAllMessagesEvent:
				//response, err = d.handleNewChat(d.ctx, ev.Data)
			default:
				msg := fmt.Sprintf("unknown event type: %T", value)
				log.Println(msg)
				err = errors.New(msg)
			}

			if err != nil {
				response = event.ErrorEvent{
					Code:    500,
					Message: err.Error(),
				}
			}

			bt, err := json.JSON.Marshal(response)
			if err != nil {
				log.Printf("fail marshaling response: %s", err)
				return
			}

			_, err = s.Write(bt)
			if err != nil {
				log.Printf("fail writing to stream: %s", err)
				return
			}
		})
	}
	return nil
}

//func (d *NodeStreamHandler) handleNewUser(ctx context.Context, data *event.Event_Data) (event.NewUserEvent, error) {
//	if ctx.Err() != nil {
//		return event.NewUserEvent{}, ctx.Err()
//	}
//	userEvent, err := data.AsNewUserEvent()
//	if err != nil {
//		return event.NewUserEvent{}, err
//	}
//	if userEvent.User == nil {
//		return event.NewUserEvent{}, nil
//	}
//	_, err = d.userRepo.Create(*userEvent.User)
//	return userEvent, err
//}
//
//func (d *NodeStreamHandler) handleNewReply(ctx context.Context, data *event.Event_Data) (reply domain.Tweet, err error) {
//	if ctx.Err() != nil {
//		return reply, ctx.Err()
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
//type RepliesResponse = dto.RepliesTreeResponse
//
//func (d *NodeStreamHandler) handleGetAllReplies(ctx context.Context, data *event.Event_Data) (response RepliesResponse, err error) {
//	if ctx.Err() != nil {
//		return response, ctx.Err()
//	}
//	event, err := data.AsGetAllRepliesEvent()
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
//func (d *NodeStreamHandler) handleGetReply(ctx context.Context, data *event.Event_Data) (domain.Tweet, error) {
//	if ctx.Err() != nil {
//		return domain.Tweet{}, ctx.Err()
//	}
//	event, err := data.AsGetReplyEvent()
//	if err != nil {
//		return domain.Tweet{}, err
//	}
//	tweet, err := d.replyRepo.GetReply(event.RootId, event.ParentReplyId, event.ReplyId)
//	if err != nil {
//		return domain.Tweet{}, err
//	}
//	return tweet, nil
//}
//
//func (d *NodeStreamHandler) handleNewTweet(ctx context.Context, data *event.Event_Data) (t domain.Tweet, err error) {
//	if ctx.Err() != nil {
//		return domain.Tweet{}, ctx.Err()
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
//
//	err = d.timelineRepo.AddTweetToTimeline(tweetEvent.Tweet.UserId, t)
//	return t, err
//}
//
//type TweetsResponse = dto.TweetsResponse
//
//func (d *NodeStreamHandler) handleGetTimeline(ctx context.Context, data *event.Event_Data) (TweetsResponse, error) {
//	if ctx.Err() != nil {
//		return TweetsResponse{}, ctx.Err()
//	}
//	ev, err := data.AsGetTimelineEvent()
//	if err != nil {
//		return TweetsResponse{}, err
//	}
//	if _, err := d.userRepo.Get(ev.UserId); err != nil && errors.Is(err, database.ErrUserNotFound) {
//		return TweetsResponse{Tweets: []domain.Tweet{}, Cursor: ""}, nil
//	}
//
//	tweets, nextCursor, err := d.timelineRepo.GetTimeline(ev.UserId, ev.Limit, ev.Cursor)
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
//func (d *NodeStreamHandler) handleGetAllTweets(ctx context.Context, data *event.Event_Data) (TweetsResponse, error) {
//	if ctx.Err() != nil {
//		return TweetsResponse{}, ctx.Err()
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
//func (d *NodeStreamHandler) handleGetTweet(ctx context.Context, data *event.Event_Data) (domain.Tweet, error) {
//	if ctx.Err() != nil {
//		return domain.Tweet{}, ctx.Err()
//	}
//	event, err := data.AsGetTweetEvent()
//	if err != nil {
//		return domain.Tweet{}, err
//	}
//	tweet, err := d.tweetRepo.Get(event.UserId, event.TweetId)
//	if err != nil {
//		return domain.Tweet{}, err
//	}
//	return tweet, nil
//}
//
//func (d *NodeStreamHandler) handleGetUser(ctx context.Context, data *event.Event_Data) (domain.User, error) {
//	if ctx.Err() != nil {
//		return domain.User{}, ctx.Err()
//	}
//
//	event, err := data.AsGetUserEvent()
//	if err != nil {
//		return domain.User{}, err
//	}
//
//	u, err := d.userRepo.Get(event.UserId)
//	if err != nil {
//		return domain.User{}, err
//	}
//	return u, nil
//}
//
//func (d *NodeStreamHandler) handleGetAllUsers(ctx context.Context, data *event.Event_Data) (r dto.UsersResponse, err error) {
//	if ctx.Err() != nil {
//		return r, ctx.Err()
//	}
//	event, err := data.AsGetAllUsersEvent()
//	if err != nil {
//		return r, err
//	}
//
//	users, cur, err := d.userRepo.List(event.Limit, event.Cursor)
//	if err != nil {
//		return r, err
//	}
//
//	return dto.UsersResponse{cur, users}, nil
//}
