package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"strings"
)

type ReplyTweetStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
}

type ReplyStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type ReplyUserFetcher interface {
	Get(userId string) (user domain.User, err error)
}

type ReplyStorer interface {
	GetReply(rootID, replyID string) (tweet domain.Tweet, err error)
	GetRepliesTree(rootID, parentId string, limit *uint64, cursor *string) ([]domain.ReplyNode, string, error)
	AddReply(reply domain.Tweet) (domain.Tweet, error)
	DeleteReply(rootID, parentID, replyID string) error
}

func StreamNewReplyHandler(
	replyRepo ReplyStorer,
	userRepo ReplyUserFetcher,
	streamer ReplyStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewReplyEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.Text == "" {
			return nil, errors.New("empty reply body")
		}
		if ev.ParentId == nil {
			return nil, errors.New("empty parent ID")
		}

		rootId := strings.TrimPrefix(ev.RootId, domain.RetweetPrefix)
		parentId := strings.TrimPrefix(*ev.ParentId, domain.RetweetPrefix)

		parentUser, err := userRepo.Get(ev.ParentUserId)
		if err != nil {
			return nil, err
		}

		reply, err := replyRepo.AddReply(domain.Tweet{
			CreatedAt: ev.CreatedAt,
			Id:        ev.Id,
			ParentId:  &parentId,
			RootId:    rootId,
			Text:      ev.Text,
			UserId:    ev.UserId,
		})
		if err != nil {
			return nil, err
		}

		replyDataResp, err := streamer.GenericStream(
			parentUser.NodeId,
			event.PUBLIC_POST_REPLY,
			event.NewReplyEvent{
				CreatedAt:    reply.CreatedAt,
				Id:           reply.Id,
				ParentId:     ev.ParentId,
				ParentUserId: ev.ParentUserId,
				RootId:       ev.RootId,
				Text:         ev.Text,
				UserId:       ev.UserId,
			},
		)
		if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(replyDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other reply error response: %s", possibleError.Message)
		}

		return reply, nil
	}
}

func StreamGetReplyHandler(repo ReplyStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetReplyEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.RootId == "" {
			return nil, errors.New("empty root id")
		}

		rootId := strings.TrimPrefix(ev.RootId, domain.RetweetPrefix)
		id := strings.TrimPrefix(ev.ReplyId, domain.RetweetPrefix)

		return repo.GetReply(rootId, id)
	}
}

func StreamDeleteReplyHandler(
	tweetRepo ReplyTweetStorer,
	userRepo ReplyUserFetcher,
	replyRepo ReplyStorer,
	streamer ReplyStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
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

		rootId := strings.TrimPrefix(ev.RootId, domain.RetweetPrefix)

		reply, err := replyRepo.GetReply(rootId, ev.ReplyId)
		if err != nil {
			return nil, err
		}

		var parentTweet domain.Tweet
		if reply.ParentId == nil {
			parentTweet, err = tweetRepo.Get(reply.UserId, rootId)
			if err != nil {
				return nil, err
			}
		} else {
			parentId := strings.TrimPrefix(*reply.ParentId, domain.RetweetPrefix)
			parentTweet, err = replyRepo.GetReply(rootId, parentId)
			if err != nil {
				return nil, err
			}
		}

		parentUser, err := userRepo.Get(parentTweet.UserId)
		if err != nil {
			return nil, err
		}

		if err = replyRepo.DeleteReply(rootId, parentTweet.Id, ev.ReplyId); err != nil {
			return nil, err
		}

		replyDataResp, err := streamer.GenericStream(
			parentUser.NodeId,
			event.PUBLIC_DELETE_REPLY,
			event.DeleteReplyEvent{
				ReplyId: ev.ReplyId,
				RootId:  rootId,
				UserId:  ev.UserId,
			},
		)
		if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(replyDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other delete reply error response: %s", possibleError.Message)
		}

		return event.Accepted, nil
	}
}

func StreamGetRepliesHandler(repo ReplyStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllRepliesEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ParentId == "" {
			return nil, errors.New("empty parent id")
		}
		if ev.RootId == "" {
			return nil, errors.New("empty root id")
		}

		rootId := strings.TrimPrefix(ev.RootId, domain.RetweetPrefix)
		parentId := strings.TrimPrefix(ev.ParentId, domain.RetweetPrefix)

		replies, cursor, err := repo.GetRepliesTree(rootId, parentId, ev.Limit, ev.Cursor)

		if err != nil {
			return nil, err
		}
		return event.RepliesResponse{
			Cursor:  cursor,
			Replies: replies,
			UserId:  &parentId,
		}, nil
	}
}
