package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type OwnerReplyStorer interface {
	GetOwner() (domain.Owner, error)
}

type ReplyBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
}

type ReplyStorer interface {
	GetReply(rootID, parentID, replyID string) (tweet domain.Tweet, err error)
	GetRepliesTree(rootID, parentID string, limit *uint64, cursor *string) ([]domain.ReplyNode, string, error)
	AddReply(reply domain.Tweet) (domain.Tweet, error)
	DeleteReply(rootID, parentID, replyID string) error
}

func StreamGetRepliesHandler(repo ReplyStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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
}

func StreamGetReplyHandler(repo ReplyStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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
}

func StreamNewReplyHandler(broadcaster ReplyBroadcaster, authRepo OwnerReplyStorer, repo ReplyStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.NewReplyEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.Text == "" {
			return nil, errors.New("empty reply body")
		}

		reply, err := repo.AddReply(domain.Tweet{
			CreatedAt: ev.CreatedAt,
			Id:        ev.Id,
			ParentId:  ev.ParentId,
			RootId:    ev.RootId,
			Text:      ev.Text,
			UserId:    ev.UserId,
			Username:  ev.Username,
		})
		if err != nil {
			return nil, err
		}
		if owner, _ := authRepo.GetOwner(); owner.UserId == ev.UserId {
			respReplyEvent := event.NewReplyEvent{
				CreatedAt: reply.CreatedAt,
				Id:        reply.Id,
				ParentId:  reply.ParentId,
				RootId:    reply.RootId,
				Text:      reply.Text,
				UserId:    reply.UserId,
				Username:  reply.Username,
			}
			reqBody := event.RequestBody{}
			_ = reqBody.FromNewReplyEvent(respReplyEvent)
			msgBody := &event.Message_Body{}
			_ = msgBody.FromRequestBody(reqBody)
			msg := event.Message{
				Body:      msgBody,
				NodeId:    owner.NodeId,
				Path:      event.PRIVATE_POST_REPLY_1_0_0,
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Infoln("broadcaster publish owner reply update:", err)
			}
		}

		return reply, nil
	}
}

func StreamDeleteReplyHandler(
	broadcaster ReplyBroadcaster,
	authRepo OwnerReplyStorer,
	repo ReplyStorer,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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
			reqBody := event.RequestBody{}
			_ = reqBody.FromDeleteReplyEvent(respReplyEvent)
			msgBody := &event.Message_Body{}
			_ = msgBody.FromRequestBody(reqBody)
			msg := event.Message{
				Body:      msgBody,
				NodeId:    owner.NodeId,
				Path:      event.PRIVATE_DELETE_REPLY_1_0_0,
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Infoln("broadcaster publish owner reply update:", err)
			}
		}
		return nil, nil
	}
}
