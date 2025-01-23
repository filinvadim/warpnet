package handler

import (
	"bytes"
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

		log.Printf("received tweets message: %s", buf.String())

		var ev event.GetAllTweetsEvent
		err = json.JSON.Unmarshal(buf.Bytes(), &ev)
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}

		tweets, cursor, err := repo.List(ev.UserId, ev.Limit, ev.Cursor)
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}

		if tweets != nil {
			response = event.TweetsResponse{
				Cursor: cursor,
				Tweets: tweets,
				UserId: ev.UserId,
			}
		}
		bt, err := json.JSON.Marshal(response)
		if err != nil {
			log.Printf("fail marshaling get tweets response: %v", err)
			return
		}

		_, err = s.Write(bt)
		if err != nil {
			log.Printf("fail writing to stream: %v", err)
		}
		bt = nil
	}
}

func StreamNewTweetHandler(repo *database.TweetRepo) func(s network.Stream) {
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

		log.Printf("received new tweet message: %s", buf.String())

		var ev event.NewTweetEvent
		err = json.JSON.Unmarshal(buf.Bytes(), &ev)
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}
		if ev.Tweet == nil {
			errResp := handleError(errors.New("tweet is nil"))
			s.Write(errResp)
			return
		}

		tweet, err := repo.Create(ev.Tweet.UserId, domain.Tweet{
			CreatedAt:     ev.Tweet.CreatedAt,
			Id:            ev.Tweet.Id,
			Likes:         ev.Tweet.Likes,
			LikesCount:    ev.Tweet.LikesCount,
			ParentId:      ev.Tweet.ParentId,
			Retweets:      ev.Tweet.Retweets,
			RetweetsCount: ev.Tweet.RetweetsCount,
			RootId:        ev.Tweet.RootId,
			Text:          ev.Tweet.Text,
			UserId:        ev.Tweet.UserId,
			Username:      ev.Tweet.Username,
		})
		if errResp := handleError(err); errResp != nil {
			s.Write(errResp)
			return
		}

		if tweet.Id != "" {
			response = tweet
		}
		bt, err := json.JSON.Marshal(response)
		if err != nil {
			log.Printf("fail marshaling new tweet response: %v", err)
			return
		}

		_, err = s.Write(bt)
		if err != nil {
			log.Printf("fail writing to stream: %v", err)
		}
		bt = nil
	}
}
