package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type RetweetedUserFetcher interface {
	GetBatch(retweetersIds ...string) (users []domain.User, err error)
}

type OwnerReTweetStorer interface {
	GetOwner() domain.Owner
}

type ReTweetBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
}

type ReTweetsStorer interface {
	NewRetweet(tweet domain.Tweet) (_ domain.Tweet, err error)
	UnRetweet(userId, tweetId string) error
	RetweetsCount(tweetId string) (uint64, error)
	Retweeters(tweetId string, limit *uint64, cursor *string) (_ []string, cur string, err error)
}

type RetweetTimelineUpdater interface {
	AddTweetToTimeline(userID string, tweet domain.Tweet) error
}

func StreamNewReTweetHandler(
	broadcaster ReTweetBroadcaster,
	authRepo OwnerReTweetStorer,
	tweetRepo ReTweetsStorer,
	timelineRepo RetweetTimelineUpdater,
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

		if owner.UserId != *retweetEvent.RetweetedBy {
			// TODO notify tweet owner about retweet
			return retweet, nil
		}

		// owner retweeted it
		if err = timelineRepo.AddTweetToTimeline(owner.UserId, retweet); err != nil {
			log.Infof("fail adding retweet to timeline: %v", err)
		}

		reqBody := event.RequestBody{}
		_ = reqBody.FromNewRetweetEvent(retweet)
		msgBody := &event.Message_Body{}
		_ = msgBody.FromRequestBody(reqBody)
		msg := event.Message{
			Body:      msgBody,
			NodeId:    owner.NodeId,
			Path:      event.PUBLIC_POST_RETWEET,
			Timestamp: time.Now(),
		}
		if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
			log.Infoln("broadcaster publish owner tweet update:", err)
		}

		return retweet, nil
	}
}

func StreamUnretweetHandler(authRepo OwnerReTweetStorer, tweetRepo ReTweetsStorer, broadcaster LikesBroadcaster) middleware.WarpHandler {
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
		err = tweetRepo.UnRetweet(ev.UserId, ev.TweetId)
		if err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()
		if ev.UserId != owner.UserId {
			// TODO notify tweet owner about unretweet
			return event.Accepted, nil
		}
		reqBody := event.RequestBody{}
		_ = reqBody.FromUnretweetEvent(ev)
		msgBody := &event.Message_Body{}
		_ = msgBody.FromRequestBody(reqBody)
		msg := event.Message{
			Body:      msgBody,
			Path:      event.PUBLIC_POST_UNRETWEET,
			Timestamp: time.Now(),
		}

		return event.Accepted, broadcaster.PublishOwnerUpdate(owner.UserId, msg)
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

		count, err := repo.RetweetsCount(ev.TweetId)
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

		retweeters, cur, err := retweetRepo.Retweeters(ev.TweetId, ev.Limit, ev.Cursor)
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
