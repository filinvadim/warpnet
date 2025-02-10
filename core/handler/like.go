package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
)

type LikedUserFetcher interface {
	GetBatch(userIDs ...string) (users []domain.User, err error)
	Get(userID string) (users domain.User, err error)
}

type LikeStreamer interface {
	GenericStream(nodeId warpnet.WarpPeerID, path stream.WarpRoute, data any) (_ []byte, err error)
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

		num, err := repo.Like(ev.TweetId, ev.UserId)
		if err != nil {
			return nil, err
		}

		likeDataResp, err := streamer.GenericStream(
			warpnet.WarpPeerID(likedUser.NodeId),
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
		if err := json.JSON.Unmarshal(likeDataResp, &possibleError); err == nil {
			return nil, fmt.Errorf("unlike stream: %s", possibleError.Message)
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

		num, err := repo.Unlike(ev.TweetId, ev.UserId)
		if err != nil {
			return nil, err
		}

		unlikeDataResp, err := streamer.GenericStream(
			warpnet.WarpPeerID(unlikedUser.NodeId),
			event.PUBLIC_POST_LIKE,
			event.UnlikeEvent{
				TweetId: ev.TweetId,
				UserId:  ev.UserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(unlikeDataResp, &possibleError); err == nil {
			return nil, fmt.Errorf("unlike stream: %s", possibleError.Message)
		}

		return event.LikesCountResponse{num}, nil
	}
}

func StreamGetLikesNumHandler(repo LikesStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetLikesCountEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}
		num, err := repo.LikesCount(ev.TweetId)
		if errors.Is(err, database.ErrLikesNotFound) {
			return event.LikesCountResponse{0}, nil
		}
		return event.LikesCountResponse{num}, err
	}
}

func StreamGetLikersHandler(likeRepo LikesStorer, userRepo LikedUserFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetLikersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}
		likers, cur, err := likeRepo.Likers(ev.TweetId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}
		if len(likers) == 0 {
			return event.GetLikersResponse{
				Cursor: "",
				Users:  []domain.User{},
			}, nil
		}

		users, err := userRepo.GetBatch(likers...)
		if err != nil {
			return nil, err
		}

		return event.GetLikersResponse{
			Cursor: cur,
			Users:  users,
		}, nil
	}
}
