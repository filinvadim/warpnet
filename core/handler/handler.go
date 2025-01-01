package handler

import (
	"errors"
	nodeGen "github.com/filinvadim/warpnet/core/node-gen"
	"github.com/filinvadim/warpnet/database"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"log"
)

func RegisterHandlers(h host.Host) error {
	sw, _ := nodeGen.GetSwagger()
	paths := sw.Paths

	if paths == nil {
		log.Fatal("swagger has no paths")
	}

	for k := range paths.Map() {
		h.SetStreamHandler(protocol.ID(k), func(s network.Stream) {
			defer s.Close()
			log.Println("new stream opened", protocol.ID(k), s.Conn().RemotePeer())

			buf := make([]byte, 1024)
			n, err := s.Read(buf)
			if err != nil {
				log.Printf("fail reading from stream: %s", err)
				return
			}
			log.Printf("Received message: %s", string(buf[:n]))

			// Отправляем ответ
			_, err = s.Write([]byte("Hello, client!"))
			if err != nil {
				log.Printf("Error writing to stream: %s", err)
				return
			}
		})
	}
	return nil
}

type NodeStreamHandler struct {
	userRepo     *database.UserRepo
	replyRepo    *database.ReplyRepo
	tweetRepo    *database.TweetRepo
	timelineRepo *database.TimelineRepo
}

func NewNodeStreamHandler() (*NodeStreamHandler, error) {
	return &NodeStreamHandler{}, nil
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

func (d *NodeStreamHandler) handleNewReply(ctx echo.Context, data *domainGen.Event_Data) (reply domainGen.Tweet, err error) {
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
func (d *NodeStreamHandler) handleGetSingleReply(ctx echo.Context, data *domainGen.Event_Data) (domainGen.Tweet, error) {
	if ctx.Request().Context().Err() != nil {
		return domainGen.Tweet{}, ctx.Request().Context().Err()
	}
	event, err := data.AsGetReplyEvent()
	if err != nil {
		return domainGen.Tweet{}, err
	}
	tweet, err := d.replyRepo.GetReply(event.RootId, event.ParentReplyId, event.ReplyId)
	if err != nil {
		return domainGen.Tweet{}, err
	}
	return tweet, nil
}

func (d *NodeStreamHandler) handleNewTweet(ctx echo.Context, data *domainGen.Event_Data) (t domainGen.Tweet, err error) {
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

func (d *NodeStreamHandler) handleGetSingleTweet(ctx echo.Context, data *domainGen.Event_Data) (domainGen.Tweet, error) {
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

func (d *NodeStreamHandler) handleGetUser(ctx echo.Context, data *domainGen.Event_Data) (domainGen.User, error) {
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
