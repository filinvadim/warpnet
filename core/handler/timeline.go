package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	warpnet "github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
)

type TimelineFetcher interface {
	GetTimeline(string, *uint64, *string) ([]domain.Tweet, string, error)
}

func StreamTimelineHandler(mr middleware.MiddlewareResolver, repo TimelineFetcher) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("timeline handler:", err)
			return
		}

		getTimelineF := func(buf []byte) (any, error) {
			var ev event.GetTimelineEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.UserId == "" {
				return nil, errors.New("empty user id")
			}

			timeline, cursor, err := repo.GetTimeline(ev.UserId, ev.Limit, ev.Cursor)
			if err != nil {
				return nil, err
			}

			if timeline != nil {
				return event.TweetsResponse{
					Cursor: cursor,
					Tweets: timeline,
					UserId: ev.UserId,
				}, nil
			}
			return nil, nil
		}

		mr.UnwrapStream(s, getTimelineF)
	}
}
