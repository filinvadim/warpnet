package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/gen/api-gen"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
	"time"
)

type OwnerTweetStorer interface {
	GetOwner() (domain.Owner, error)
}

type TweetBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg api.Message) (err error)
}

type TweetsStorer interface {
	Get(userID, tweetID string) (tweet domain.Tweet, err error)
	List(string, *uint64, *string) ([]domain.Tweet, string, error)
	Create(_ string, tweet domain.Tweet) (domain.Tweet, error)
	Delete(userID, tweetID string) error
}

type TimelineUpdater interface {
	AddTweetToTimeline(userID string, tweet domain.Tweet) error
}

func StreamGetTweetsHandler(repo TweetsStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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
	return func(buf []byte) (any, error) {
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
	return func(buf []byte) (any, error) {
		var ev event.NewTweetEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}

		tweet, err := tweetRepo.Create(ev.UserId, domain.Tweet{
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

		if tweet.Id == "" {
			return tweet, errors.New("tweet handler: empty tweet id")
		}
		if err = timelineRepo.AddTweetToTimeline(tweet.UserId, tweet); err != nil {
			log.Printf("fail adding tweet to timeline: %v", err)
		}
		if owner, _ := authRepo.GetOwner(); owner.UserId == ev.UserId {
			respTweetEvent := event.NewTweetEvent{
				CreatedAt:     tweet.CreatedAt,
				Id:            tweet.Id,
				Likes:         tweet.Likes,
				LikesCount:    tweet.LikesCount,
				ParentId:      tweet.ParentId,
				Retweets:      tweet.Retweets,
				RetweetsCount: tweet.RetweetsCount,
				RootId:        tweet.RootId,
				Text:          tweet.Text,
				UserId:        tweet.UserId,
				Username:      tweet.Username,
			}
			respEvent := &api.Event{}
			_ = respEvent.FromNewTweetEvent(respTweetEvent)
			msg := api.Message{
				Data:      respEvent,
				NodeId:    owner.NodeId,
				Path:      stream.TweetPostPrivate.String(),
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Println("broadcaster publish owner tweet update:", err)
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
	return func(buf []byte) (any, error) {
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
		if owner, _ := authRepo.GetOwner(); owner.UserId == ev.UserId {
			respTweetEvent := event.DeleteTweetEvent{
				UserId:  ev.UserId,
				TweetId: ev.TweetId,
			}
			respEvent := &api.Event{}
			_ = respEvent.FromDeleteTweetEvent(respTweetEvent)
			msg := api.Message{
				Data:      respEvent,
				NodeId:    owner.NodeId,
				Path:      stream.TweetDeletePrivate.String(),
				Timestamp: time.Now(),
			}
			if err := broadcaster.PublishOwnerUpdate(owner.UserId, msg); err != nil {
				log.Println("broadcaster publish owner tweet update:", err)
			}
		}
		return nil, nil
	}
}
