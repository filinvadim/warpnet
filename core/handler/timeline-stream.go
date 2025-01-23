package handler

import (
	"bytes"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
	"log"
)

func StreamTimelineHandler(repo *database.TimelineRepo) func(s network.Stream) {
	return func(s network.Stream) {
		defer s.Close()

		log.Println("stream opened", s.Protocol(), s.Conn().RemotePeer())

		var (
			response any
			err      error
		)

		buf := bytes.NewBuffer(nil)
		_, err = buf.ReadFrom(s)
		if err != nil {
			log.Printf("fail reading from stream: %s", err)
			return
		}

		log.Printf("received timeline message: %s", buf.String())

		var ev event.GetTimelineEvent
		err = json.JSON.Unmarshal(buf.Bytes(), &ev)
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}

		timeline, cursor, err := repo.GetTimeline(ev.UserId, ev.Limit, ev.Cursor)
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}

		if timeline != nil {
			response = event.TweetsResponse{
				Cursor: cursor,
				Tweets: timeline,
				UserId: ev.UserId,
			}
		}
		bt, err := json.JSON.Marshal(response)
		if err != nil {
			log.Printf("fail marshaling timeline response: %v", err)
			return
		}

		_, err = s.Write(bt)
		if err != nil {
			log.Printf("fail writing to stream: %v", err)
		}
		bt = nil
	}
}
