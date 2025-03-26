package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"strings"
)

type RetweetStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type RetweetedUserFetcher interface {
	GetBatch(retweetersIds ...string) (users []domain.User, err error)
	Get(userId string) (users domain.User, err error)
}

type OwnerReTweetStorer interface {
	GetOwner() domain.Owner
}

type ReTweetsStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
	NewRetweet(tweet domain.Tweet) (_ domain.Tweet, err error)
	UnRetweet(retweetedByUserID, tweetId string) error
	RetweetsCount(tweetId string) (uint64, error)
	Retweeters(tweetId string, limit *uint64, cursor *string) (_ []string, cur string, err error)
}

type RetweetTimelineUpdater interface {
	AddTweetToTimeline(userId string, tweet domain.Tweet) error
}

func StreamNewReTweetHandler(
	authRepo OwnerReTweetStorer,
	userRepo RetweetedUserFetcher,
	tweetRepo ReTweetsStorer,
	timelineRepo RetweetTimelineUpdater,
	streamer RetweetStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var retweetEvent event.NewRetweetEvent
		err := json.JSON.Unmarshal(buf, &retweetEvent)
		if err != nil {
			return nil, err
		}
		if retweetEvent.RetweetedBy == nil {
			return nil, errors.New("retweeted by unknown")
		}
		if retweetEvent.Id == "" {
			return nil, errors.New("empty retweet id")
		}

		retweet, err := tweetRepo.NewRetweet(retweetEvent)
		if err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()
		if owner.UserId == *retweetEvent.RetweetedBy {
			// owner retweeted it
			if err = timelineRepo.AddTweetToTimeline(owner.UserId, retweet); err != nil {
				log.Infof("fail adding retweet to timeline: %v", err)
			}
			return retweet, nil
		}

		tweetOwner, err := userRepo.Get(retweetEvent.UserId)
		if err != nil {
			return nil, err
		}

		retweetDataResp, err := streamer.GenericStream(
			tweetOwner.NodeId,
			event.PUBLIC_POST_RETWEET,
			event.NewRetweetEvent(retweet),
		)
		if err != nil {
			return nil, err
		}
		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(retweetDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other retweet error response: %s", possibleError.Message)
		}

		return retweet, nil
	}
}

func StreamUnretweetHandler(
	authRepo OwnerReTweetStorer,
	tweetRepo ReTweetsStorer,
	userRepo RetweetedUserFetcher,
	streamer RetweetStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.UnretweetEvent
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

		retweetedBy := ev.UserId

		tweet, err := tweetRepo.Get(retweetedBy, ev.TweetId)
		if err != nil {
			return nil, err
		}
		err = tweetRepo.UnRetweet(retweetedBy, ev.TweetId)
		if err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()
		if tweet.UserId == owner.UserId {
			// tweet belongs to owner, unretweet themself
			return event.Accepted, nil
		}

		tweetOwner, err := userRepo.Get(tweet.UserId)
		if err != nil {
			return nil, err
		}

		unretweetDataResp, err := streamer.GenericStream(
			tweetOwner.NodeId,
			event.PUBLIC_POST_UNRETWEET,
			ev,
		)
		if err != nil {
			return nil, err
		}
		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(unretweetDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other unretweet error response: %s", possibleError.Message)
		}

		return event.Accepted, nil
	}
}

func StreamGetReTweetsCountHandler(repo ReTweetsStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetReTweetsCountEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}

		count, err := repo.RetweetsCount(strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix))
		if err != nil {
			return nil, err
		}
		return event.ReTweetsCountResponse{Count: count}, nil
	}
}

func StreamGetRetweetersHandler(
	retweetRepo ReTweetsStorer,
	userRepo RetweetedUserFetcher,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetRetweetersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}

		retweeters, cur, err := retweetRepo.Retweeters(
			strings.TrimPrefix(ev.TweetId, domain.RetweetPrefix), ev.Limit, ev.Cursor,
		)
		if err != nil {
			return nil, err
		}

		users, err := userRepo.GetBatch(retweeters...)
		if err != nil && !errors.Is(err, database.ErrUserNotFound) {
			return nil, err
		}

		return event.GetRetweetersResponse{
			Cursor: cur,
			Users:  users,
		}, nil
	}
}
