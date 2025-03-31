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

type LikedUserFetcher interface {
	GetBatch(userIds ...string) (users []domain.User, err error)
	Get(userId string) (users domain.User, err error)
}

type LikeStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type LikesStorer interface {
	Like(tweetId, userId string) (likesNum uint64, err error)
	Unlike(tweetId, userId string) (likesNum uint64, err error)
	LikesCount(tweetId string) (likesNum uint64, err error)
	Likers(tweetId string, limit *uint64, cursor *string) (likers []string, cur string, err error)
}

func StreamLikeHandler(repo LikesStorer, userRepo LikedUserFetcher, streamer LikeStreamer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.LikeEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		if ev.UserId == "" {
			return nil, errors.New("like: empty user id")
		}
		if ev.TweetId == "" {
			return nil, errors.New("like: empty tweet id")
		}

		likedUser, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		num, err := repo.Like(strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix), ev.UserId)
		if err != nil {
			return nil, err
		}

		likeDataResp, err := streamer.GenericStream(
			likedUser.NodeId,
			event.PUBLIC_POST_LIKE,
			event.LikeEvent{
				TweetId: ev.TweetId,
				UserId:  ev.UserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(likeDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other like error response: %s", possibleError.Message)
		}

		return event.LikesCountResponse{num}, nil
	}
}

func StreamUnlikeHandler(repo LikesStorer, userRepo LikedUserFetcher, streamer LikeStreamer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.UnlikeEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}

		unlikedUser, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		num, err := repo.Unlike(ev.UserId, strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix))
		if err != nil {
			return nil, err
		}

		unlikeDataResp, err := streamer.GenericStream(
			unlikedUser.NodeId,
			event.PUBLIC_POST_UNLIKE,
			event.UnlikeEvent{
				TweetId: ev.TweetId,
				UserId:  ev.UserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(unlikeDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other unlike error response: %s", possibleError.Message)
		}

		return event.LikesCountResponse{num}, nil
	}
}
