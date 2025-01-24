package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
)

func StreamTimelineHandler(repo *database.TimelineRepo) func(s network.Stream) {
	return func(s network.Stream) {
		ReadStream(s, func(buf []byte) (any, error) {
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
		})
	}
}
