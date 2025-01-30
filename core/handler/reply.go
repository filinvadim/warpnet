package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/api-gen"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
	"time"
)

type OwnerReplyStorer interface {
	GetOwner() (domain.Owner, error)
}

type ReplyBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg api.Message) (err error)
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
		getReplyF := func(buf []byte) (any, error) {
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
		mr.UnwrapStream(s, getReplyF)
	}
}

func StreamNewReplyHandler(
	mr middleware.MiddlewareResolver,
	broadcaster ReplyBroadcaster,
	authRepo OwnerReplyStorer,
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
			if ev.Text == "" {
				return nil, errors.New("empty reply body")
			}

			reply, err := repo.AddReply(domain.Tweet{
				CreatedAt:     ev.CreatedAt,
				Id:            ev.Id,
				Likes:         ev.Likes,
				LikesCount:    ev.LikesCount,
				ParentId:      ev.ParentId,
				Retweets:      ev.Retweets,
				RetweetsCount: ev.RetweetsCount,
				RootId:        ev.RootId,
				Text:          ev.Text,
				UserId:        ev.UserId,
				Username:      ev.Username,
			})
			if err != nil {
				return nil, err
			}
			if owner, _ := authRepo.GetOwner(); owner.UserId == ev.UserId {
				respReplyEvent := event.NewReplyEvent{
					CreatedAt:     reply.CreatedAt,
					Id:            reply.Id,
					Likes:         reply.Likes,
					LikesCount:    reply.LikesCount,
					ParentId:      reply.ParentId,
					Retweets:      reply.Retweets,
					RetweetsCount: reply.RetweetsCount,
					RootId:        reply.RootId,
					Text:          reply.Text,
					UserId:        reply.UserId,
					Username:      reply.Username,
				}
				respEvent := &api.Event{}
				_ = respEvent.FromNewReplyEvent(respReplyEvent)
				msg := api.Message{
					Data:      respEvent,
					NodeId:    owner.NodeId,
					Path:      stream.ReplyPostPrivate.String(),
					Timestamp: time.Now(),
				}
				if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
					log.Println("broadcaster publish owner reply update:", err)
				}
			}

			return reply, nil
		}
		mr.UnwrapStream(s, addF)
	}
}

func StreamDeleteReplyHandler(
	mr middleware.MiddlewareResolver,
	broadcaster ReplyBroadcaster,
	authRepo OwnerReplyStorer,
	repo ReplyStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("delete tweet handler:", err)
			return
		}
		delReplyF := func(buf []byte) (any, error) {
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

			if err = repo.DeleteReply(ev.RootId, ev.ParentReplyId, ev.ReplyId); err != nil {
				return nil, err
			}
			if owner, _ := authRepo.GetOwner(); owner.UserId == ev.UserId {
				respReplyEvent := event.DeleteReplyEvent{
					UserId:        ev.UserId,
					ReplyId:       ev.ReplyId,
					ParentReplyId: ev.ParentReplyId,
					RootId:        ev.RootId,
				}
				respEvent := &api.Event{}
				_ = respEvent.FromDeleteReplyEvent(respReplyEvent)
				msg := api.Message{
					Data:      respEvent,
					NodeId:    owner.NodeId,
					Path:      stream.ReplyDeletePrivate.String(),
					Timestamp: time.Now(),
				}
				if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
					log.Println("broadcaster publish owner reply update:", err)
				}
			}
			return nil, nil
		}
		mr.UnwrapStream(s, delReplyF)
	}
}
