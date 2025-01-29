package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/server/api-gen"
	"log"
)

type TweetBroadcaster interface {
	PublishOwnerUpdate(owner domain.Owner, msg api.Message) (err error)
	SubscribeUserUpdate(user domain.User) (err error)
	UnsubscribeUserUpdate(user domain.User) (err error)
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

func StreamGetTweetsHandler(
	mr middleware.MiddlewareResolver, repo TweetsStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		getTweetsF := func(buf []byte) (any, error) {
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
		mr.UnwrapStream(s, getTweetsF)
	}
}

func StreamGetTweetHandler(mr middleware.MiddlewareResolver, repo TweetsStorer) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		getTweetF := func(buf []byte) (any, error) {
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
		mr.UnwrapStream(s, getTweetF)
	}
}

func StreamNewTweetHandler(
	mr middleware.MiddlewareResolver,
	broadcaster TweetBroadcaster,
	tweetRepo TweetsStorer,
	timelineRepo TimelineUpdater,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("new tweet handler:", err)
			return
		}

		createF := func(buf []byte) (any, error) {
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
			return tweet, nil
		}
		mr.UnwrapStream(s, createF)
	}
}

func StreamDeleteTweetHandler(
	mr middleware.MiddlewareResolver, broadcaster TweetBroadcaster, repo TweetsStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("delete tweet handler:", err)
			return
		}
		delTweetF := func(buf []byte) (any, error) {
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

			return nil, repo.Delete(ev.UserId, ev.TweetId)
		}
		mr.UnwrapStream(s, delTweetF)
	}
}
