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
	log "github.com/sirupsen/logrus"
	"time"
)

type TweetUserFetcher interface {
	Get(userId string) (user domain.User, err error)
}

type TweetStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
}

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

func StreamGetTweetsHandler(
	repo TweetsStorer,
	authRepo OwnerTweetStorer,
	userRepo TweetUserFetcher,
	streamer TweetStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllTweetsEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}

		ownerId := authRepo.GetOwner().UserId
		if ev.UserId == ownerId {
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
		}

		otherUser, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, fmt.Errorf("other user get: %v", err)
		}

		tweetsDataResp, err := streamer.GenericStream(
			otherUser.NodeId,
			event.PUBLIC_GET_TWEETS,
			ev,
		)
		if err != nil {
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
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(tweetsDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other tweets error response: %s", possibleError.Message)
		}

		var tweetsResp event.TweetsResponse
		if err := json.JSON.Unmarshal(tweetsDataResp, &tweetsResp); err != nil {
			return nil, err
		}

		return tweetsResp, nil
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
