package handler

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/node"
	nodeGen "github.com/filinvadim/warpnet/core/node-gen"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	eventGen "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
)

type NodeStreamHandler struct {
	userRepo     *database.UserRepo
	replyRepo    *database.ReplyRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
}

func NewNodeStreamHandler() (*NodeStreamHandler, error) {
	return &NodeStreamHandler{}, nil
}

func (d *NodeStreamHandler) RegisterHandlers(h node.P2PNode) error {
	sw, _ := nodeGen.GetSwagger()
	paths := sw.Paths

	if paths == nil {
		log.Fatal("swagger has no paths")
	}

	for k := range paths.Map() {
		h.SetStreamHandler(node.WarpDiscriminator(k), func(s network.Stream) {
			defer s.Close()
			log.Println("new stream opened", node.WarpDiscriminator(k), s.Conn().RemotePeer())

			buf := bytes.NewBuffer(nil)
			_, err := buf.ReadFrom(s)
			if err != nil {
				log.Printf("fail reading from stream: %s", err)
				return
			}

			log.Printf("Received message: %s", buf.String())
			var ev eventGen.Event
			err = json.JSON.Unmarshal(buf.Bytes(), &ev)
			if err != nil {
				log.Printf("fail unmarshaling event: %s", err)
				return
			}
			value, err := ev.Data.ValueByDiscriminator()
			if err != nil {
				log.Printf("fail getting discriminator value: %s", err)
				return
			}

			var response any

			switch value.(type) {
			case eventGen.NewUserEvent:
			case eventGen.GetUserEvent:
			case eventGen.GetAllUsersEvent:
			case eventGen.NewChatEvent:
			case eventGen.GetChatEvent:
			case eventGen.GetAllChatsEvent:
			case eventGen.NewTweetEvent:
			case eventGen.GetTweetEvent:
			case eventGen.GetAllTweetsEvent:
			case eventGen.GetTimelineEvent:
			case eventGen.NewReplyEvent:
			case eventGen.GetReplyEvent:
			case eventGen.GetAllRepliesEvent:
			case eventGen.NewMessageEvent:
			case eventGen.GetMessageEvent:
			case eventGen.GetAllMessagesEvent:
			default:
				msg := fmt.Sprintf("unknown event type: %T", value)
				log.Println(msg)
				response = eventGen.ErrorEvent{
					Code:    500,
					Message: msg,
				}
			}

			bt, err := json.JSON.Marshal(response)
			if err != nil {
				log.Printf("fail marshaling response: %s", err)
				return
			}

			_, err = s.Write(bt)
			if err != nil {
				log.Printf("Error writing to stream: %s", err)
				return
			}
		})
	}
	return nil
}

func (d *NodeStreamHandler) handleNewUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.NewUserEvent, error) {
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

func (d *NodeStreamHandler) handleNewReply(ctx echo.Context, data *domainGen.Event_Data) (reply domain.Tweet, err error) {
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

func (d *NodeStreamHandler) handleGetReplies(ctx echo.Context, data *domainGen.Event_Data) (response RepliesResponse, err error) {
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
func (d *NodeStreamHandler) handleGetSingleReply(ctx echo.Context, data *domainGen.Event_Data) (domain.Tweet, error) {
	if ctx.Request().Context().Err() != nil {
		return domain.Tweet{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetReplyEvent()
	if err != nil {
		return domain.Tweet{}, err
	}
	tweet, err := d.replyRepo.GetReply(event.RootId, event.ParentReplyId, event.ReplyId)
	if err != nil {
		return domain.Tweet{}, err
	}
	return tweet, nil
}

func (d *NodeStreamHandler) handleNewTweet(ctx echo.Context, data *domainGen.Event_Data) (t domain.Tweet, err error) {
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

	//if myOwnTweet {
	//	err = d.timelineRepo.AddTweetToTimeline(tweetEvent.Tweet.UserId, t)
	//}
	return t, err
}

type TweetsResponse = domainGen.TweetsResponse

func (d *NodeStreamHandler) handleGetTimeline(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
	if ctx.Request().Context().Err() != nil {
		return TweetsResponse{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTimelineEvent()
	if err != nil {
		return TweetsResponse{}, err
	}
	if _, err := d.userRepo.Get(event.UserId); err != nil && errors.Is(err, database.ErrUserNotFound) {
		return TweetsResponse{Tweets: []domain.Tweet{}, Cursor: ""}, nil
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

func (d *NodeStreamHandler) handleGetTweets(ctx echo.Context, data *domainGen.Event_Data) (TweetsResponse, error) {
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

func (d *NodeStreamHandler) handleGetSingleTweet(ctx echo.Context, data *domainGen.Event_Data) (domain.Tweet, error) {
	if ctx.Request().Context().Err() != nil {
		return domain.Tweet{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetTweetEvent()
	if err != nil {
		return domain.Tweet{}, err
	}
	tweet, err := d.tweetRepo.Get(event.UserId, event.TweetId)
	if err != nil {
		return domain.Tweet{}, err
	}
	return tweet, nil
}

func (d *NodeStreamHandler) handleGetUser(ctx echo.Context, data *domainGen.Event_Data) (domain.User, error) {
	if ctx.Request().Context().Err() != nil {
		return domain.User{}, ctx.Request().Context().Err()
	}

	event, err := data.AsGetUserEvent()
	if err != nil {
		return domain.User{}, err
	}

	u, err := d.userRepo.Get(event.UserId)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (d *NodeStreamHandler) handleGetAllUsers(ctx echo.Context, data *domainGen.Event_Data) (domainGen.UsersResponse, error) {
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
