package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
)

func StreamGetTweetsHandler(repo *database.TweetRepo) func(s network.Stream) {
	return func(s network.Stream) {
		ReadStream(s, func(buf []byte) (any, error) {

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
		})
	}
}

func StreamNewTweetHandler(
	tweetRepo *database.TweetRepo, timelineRepo *database.TimelineRepo,
) func(s network.Stream) {
	return func(s network.Stream) {
		ReadStream(s, func(buf []byte) (any, error) {
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

			if tweet.Id != "" {
				if err = timelineRepo.AddTweetToTimeline(tweet.UserId, tweet); err != nil {
					log.Printf("fail adding tweet to timeline: %v", err)
				}
				return tweet, nil
			}
			return nil, err
		})
	}
}
