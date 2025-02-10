package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type OwnerTweetStorer interface {
	GetOwner() domain.Owner
}

type TweetBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
}

type TweetsStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
	List(string, *uint64, *string) ([]domain.Tweet, string, error)
	Create(_ string, tweet domain.Tweet) (domain.Tweet, error)
	Delete(userID, tweetID string) error
}

type TimelineUpdater interface {
	AddTweetToTimeline(userId string, tweet domain.Tweet) error
}

func StreamGetTweetsHandler(repo TweetsStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllTweetsEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		tweets, cursor, err := repo.List(ev.UserId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		if tweets != nil {
			return event.TweetsResponse{
				Cursor: cursor,
				Tweets: tweets,
				UserId: ev.UserId,
			}, nil
		}
		return nil, nil
	}
}

func StreamGetTweetHandler(repo TweetsStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetTweetEvent
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

		return repo.Get(ev.UserId, ev.TweetId)
	}
}

func StreamNewTweetHandler(
	broadcaster TweetBroadcaster,
	authRepo OwnerTweetStorer,
	tweetRepo TweetsStorer,
	timelineRepo TimelineUpdater,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewTweetEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}

		tweet, err := tweetRepo.Create(ev.UserId, ev)
		if err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()

		if tweet.Id == "" {
			return tweet, errors.New("tweet handler: empty tweet id")
		}
		if err = timelineRepo.AddTweetToTimeline(owner.UserId, tweet); err != nil {
			log.Infof("fail adding tweet to timeline: %v", err)
		}
		if owner.UserId == ev.UserId {
			respTweetEvent := event.NewTweetEvent{
				CreatedAt: tweet.CreatedAt,
				Id:        tweet.Id,
				ParentId:  tweet.ParentId,
				RootId:    tweet.RootId,
				Text:      tweet.Text,
				UserId:    tweet.UserId,
				Username:  tweet.Username,
			}
			reqBody := event.RequestBody{}
			_ = reqBody.FromNewTweetEvent(respTweetEvent)
			msgBody := &event.Message_Body{}
			_ = msgBody.FromRequestBody(reqBody)
			msg := event.Message{
				Body:      msgBody,
				NodeId:    owner.NodeId,
				Path:      event.PRIVATE_POST_TWEET,
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Infoln("broadcaster publish owner tweet update:", err)
			}
		}
		return tweet, nil
	}
}

func StreamDeleteTweetHandler(
	broadcaster TweetBroadcaster,
	authRepo OwnerTweetStorer,
	repo TweetsStorer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.DeleteTweetEvent
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

		if err := repo.Delete(ev.UserId, ev.TweetId); err != nil {
			return nil, err
		}
		owner := authRepo.GetOwner()
		if owner.UserId == ev.UserId {
			respTweetEvent := event.DeleteTweetEvent{
				UserId:  ev.UserId,
				TweetId: ev.TweetId,
			}
			reqBody := event.RequestBody{}
			_ = reqBody.FromDeleteTweetEvent(respTweetEvent)
			msgBody := &event.Message_Body{}
			_ = msgBody.FromRequestBody(reqBody)
			msg := event.Message{
				Body:      msgBody,
				NodeId:    owner.NodeId,
				Path:      event.PRIVATE_DELETE_TWEET,
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Infoln("broadcaster publish owner tweet update:", err)
			}
		}

		return nil, nil
	}
}
