package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/api-gen"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
)

type ReplyBroadcaster interface {
	PublishOwnerUpdate(owner domain.Owner, msg api.api) (err error)
	SubscribeUserUpdate(user domain.User) (err error)
	UnsubscribeUserUpdate(user domain.User) (err error)
}

type ReplyStorer interface {
	GetReply(rootID, parentID, replyID string) (tweet domain.Tweet, err error)
	GetRepliesTree(rootID, parentID string, limit *uint64, cursor *string) ([]domain.ReplyNode, string, error)
	AddReply(reply domain.Tweet) (domain.Tweet, error)
	DeleteReply(rootID, parentID, replyID string) error
}

func StreamGetRepliesHandler(
	mr middleware.MiddlewareResolver, repo ReplyStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		getAllF := func(buf []byte) (any, error) {
			var ev event.GetAllRepliesEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.ParentReplyId == "" {
				return nil, errors.New("empty parent id")
			}
			if ev.RootId == "" {
				return nil, errors.New("empty root id")
			}
			replies, cursor, err := repo.GetRepliesTree(ev.RootId, ev.ParentReplyId, ev.Limit, ev.Cursor)
			if err != nil {
				return nil, err
			}
			return event.RepliesTreeResponse{
				Cursor:  cursor,
				Replies: replies,
			}, nil

		}
		mr.UnwrapStream(s, getAllF)
	}
}

func StreamGetReplyHandler(mr middleware.MiddlewareResolver, repo ReplyStorer) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		getTweetF := func(buf []byte) (any, error) {
			var ev event.GetReplyEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.ParentReplyId == "" {
				return nil, errors.New("empty parent id")
			}
			if ev.RootId == "" {
				return nil, errors.New("empty root id")
			}

			return repo.GetReply(ev.RootId, ev.ParentReplyId, ev.ReplyId)
		}
		mr.UnwrapStream(s, getTweetF)
	}
}

func StreamNewReplyHandler(
	mr middleware.MiddlewareResolver,
	broadcaster ReplyBroadcaster,
	repo ReplyStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("new tweet handler:", err)
			return
		}

		addF := func(buf []byte) (any, error) {
			var ev event.NewReplyEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.Tweet == nil {
				return nil, errors.New("empty reply body")
			}

			return repo.AddReply(domain.Tweet{
				CreatedAt:     ev.Tweet.CreatedAt,
				Id:            ev.Tweet.Id,
				Likes:         ev.Tweet.Likes,
				LikesCount:    ev.Tweet.LikesCount,
				ParentId:      ev.Tweet.ParentId,
				Retweets:      ev.Tweet.Retweets,
				RetweetsCount: ev.Tweet.RetweetsCount,
				RootId:        ev.Tweet.RootId,
				Text:          ev.Tweet.Text,
				UserId:        ev.Tweet.UserId,
				Username:      ev.Tweet.Username,
			})
		}
		mr.UnwrapStream(s, addF)
	}
}

func StreamDeleteReplyHandler(
	mr middleware.MiddlewareResolver, broadcaster ReplyBroadcaster, repo ReplyStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("delete tweet handler:", err)
			return
		}
		delTweetF := func(buf []byte) (any, error) {
			var ev event.DeleteReplyEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.ReplyId == "" {
				return nil, errors.New("empty reply id")
			}
			if ev.RootId == "" {
				return nil, errors.New("empty root id")
			}
			if ev.ParentReplyId == "" {
				return nil, errors.New("empty parent id")
			}

			return nil, repo.DeleteReply(ev.RootId, ev.ParentReplyId, ev.ReplyId)
		}
		mr.UnwrapStream(s, delTweetF)
	}
}
