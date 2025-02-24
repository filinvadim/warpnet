package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/vmihailenco/msgpack/v5"
)

type TimelineFetcher interface {
	GetTimeline(string, *uint64, *string) ([]domain.Tweet, string, error)
}

func StreamTimelineHandler(repo TimelineFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetTimelineEvent
		err := msgpack.Unmarshal(buf, &ev)
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
}
