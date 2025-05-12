// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"strings"
)

type LikeTweetsStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
	List(string, *uint64, *string) ([]domain.Tweet, string, error)
	Create(_ string, tweet domain.Tweet) (domain.Tweet, error)
	Delete(userID, tweetID string) error
}

type LikedUserFetcher interface {
	GetBatch(userIds ...string) (users []domain.User, err error)
	Get(userId string) (users domain.User, err error)
}

type LikeStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
	NodeInfo() warpnet.NodeInfo
}

type LikesStorer interface {
	Like(tweetId, userId string) (likesNum uint64, err error)
	Unlike(tweetId, userId string) (likesNum uint64, err error)
	LikesCount(tweetId string) (likesNum uint64, err error)
	Likers(tweetId string, limit *uint64, cursor *string) (likers []string, cur string, err error)
}

func StreamLikeHandler(
	repo LikesStorer,
	userRepo LikedUserFetcher,
	streamer LikeStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.LikeEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.OwnerId == "" {
			return nil, warpnet.WarpError("like: empty owner id")
		}
		if ev.UserId == "" {
			return nil, warpnet.WarpError("like: empty user id")
		}
		if ev.TweetId == "" {
			return nil, warpnet.WarpError("like: empty tweet id")
		}

		tweetId := strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix)
		num, err := repo.Like(tweetId, ev.OwnerId) // store my like
		if err != nil {
			return nil, err
		}

		likedUser, err := userRepo.Get(ev.UserId) // get other user info
		if err != nil {
			return nil, err
		}

		if ev.OwnerId == ev.UserId { // own tweet like
			return event.LikesCountResponse{num}, nil
		}
		if ev.OwnerId != streamer.NodeInfo().OwnerId { // like exchange finished
			return event.LikesCountResponse{num}, nil
		}

		likeDataResp, err := streamer.GenericStream(
			likedUser.NodeId,
			event.PUBLIC_POST_LIKE,
			event.LikeEvent{
				TweetId: ev.TweetId,
				OwnerId: ev.OwnerId,
				UserId:  ev.UserId,
			},
		)
		if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(likeDataResp, &possibleError); possibleError.Message != "" {
			log.Errorf("unmarshal other like error response: %s", possibleError.Message)
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
			return nil, warpnet.WarpError("empty user id")
		}
		if ev.TweetId == "" {
			return nil, warpnet.WarpError("empty tweet id")
		}

		unlikedUser, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		tweetId := strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix)
		num, err := repo.Unlike(tweetId, ev.OwnerId)
		if err != nil {
			return nil, err
		}

		if ev.OwnerId == ev.UserId { // own tweet dislike
			return event.LikesCountResponse{num}, nil
		}
		if ev.OwnerId != streamer.NodeInfo().OwnerId { // dislike exchange finished
			return event.LikesCountResponse{num}, nil
		}

		unlikeDataResp, err := streamer.GenericStream(
			unlikedUser.NodeId,
			event.PUBLIC_POST_UNLIKE,
			event.UnlikeEvent{
				TweetId: ev.TweetId,
				UserId:  ev.UserId,
				OwnerId: ev.OwnerId,
			},
		)
		if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(unlikeDataResp, &possibleError); possibleError.Message != "" {
			log.Errorf("unmarshal other unlike error response: %s", possibleError.Message)
		}

		return event.LikesCountResponse{num}, nil
	}
}
