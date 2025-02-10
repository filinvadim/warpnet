package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
)

type ReplyTweetStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
}

type ReplyStreamer interface {
	GenericStream(nodeId warpnet.WarpPeerID, path stream.WarpRoute, data any) (_ []byte, err error)
}

type ReplyUserFetcher interface {
	Get(userId string) (user domain.User, err error)
}

type ReplyStorer interface {
	GetReply(rootID, replyID string) (tweet domain.Tweet, err error)
	GetRepliesTree(rootID string, limit *uint64, cursor *string) ([]domain.ReplyNode, string, error)
	AddReply(reply domain.Tweet) (domain.Tweet, error)
	DeleteReply(rootID, replyID string) error
}

func StreamNewReplyHandler(
	replyRepo ReplyStorer,
	userRepo ReplyUserFetcher,
	tweetRepo ReplyTweetStorer,
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

		var parentTweet domain.Tweet
		if ev.ParentId == nil {
			parentTweet, err = tweetRepo.Get(ev.UserId, ev.RootId)
			if err != nil {
				return nil, err
			}
		} else {
			parentTweet, err = replyRepo.GetReply(ev.RootId, *ev.ParentId)
			if err != nil {
				return nil, err
			}
		}

		parentUser, err := userRepo.Get(parentTweet.UserId)
		if err != nil {
			return nil, err
		}

		reply, err := replyRepo.AddReply(domain.Tweet{
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

		replyDataResp, err := streamer.GenericStream(
			warpnet.WarpPeerID(parentUser.NodeId),
			event.PUBLIC_POST_REPLY,
			event.NewReplyEvent(reply),
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(replyDataResp, &possibleError); err == nil {
			return nil, fmt.Errorf("new reply stream: %s", possibleError.Message)
		}

		return reply, nil
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

		reply, err := replyRepo.GetReply(ev.RootId, ev.ReplyId)
		if err != nil {
			return nil, err
		}

		var parentTweet domain.Tweet
		if reply.ParentId == nil {
			parentTweet, err = tweetRepo.Get(reply.UserId, reply.RootId)
			if err != nil {
				return nil, err
			}
		} else {
			parentTweet, err = replyRepo.GetReply(reply.RootId, *reply.ParentId)
			if err != nil {
				return nil, err
			}
		}

		parentUser, err := userRepo.Get(parentTweet.UserId)
		if err != nil {
			return nil, err
		}

		if err = replyRepo.DeleteReply(ev.RootId, ev.ReplyId); err != nil {
			return nil, err
		}

		replyDataResp, err := streamer.GenericStream(
			warpnet.WarpPeerID(parentUser.NodeId),
			event.PUBLIC_DELETE_REPLY,
			event.DeleteReplyEvent{
				ReplyId: ev.ReplyId,
				RootId:  ev.RootId,
				UserId:  ev.UserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(replyDataResp, &possibleError); err == nil {
			return nil, fmt.Errorf("new reply stream: %s", possibleError.Message)
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
		replies, cursor, err := repo.GetRepliesTree(ev.RootId, ev.Limit, ev.Cursor)
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
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetReplyEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.RootId == "" {
			return nil, errors.New("empty root id")
		}

		return repo.GetReply(ev.RootId, ev.ReplyId)
	}
}
