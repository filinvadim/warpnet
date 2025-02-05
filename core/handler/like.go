package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"time"
)

type LikedUserFetcher interface {
	GetBatch(userIDs ...string) (users []domain.User, err error)
}

type LikesBroadcaster interface {
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
}

type LikesStorer interface {
	Like(tweetId, userId string) (likesNum int64, err error)
	Unlike(tweetId, userId string) (likesNum int64, err error)
	LikesCount(tweetId string) (likesNum int64, err error)
	Likers(tweetId string, limit *uint64, cursor *string) (likers []string, cur string, err error)
}

func StreamLikeHandler(repo LikesStorer, broadcaster LikesBroadcaster) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.LikeEvent
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
		num, err := repo.Like(ev.TweetId, ev.UserId)
		if err != nil {
			return nil, err
		}

		reqBody := event.RequestBody{}
		_ = reqBody.FromLikeEvent(ev)
		msgBody := &event.Message_Body{}
		_ = msgBody.FromRequestBody(reqBody)
		msg := event.Message{
			Body:      msgBody,
			Path:      event.PRIVATE_POST_LIKE,
			Timestamp: time.Now(),
		}

		return event.LikesNumResponse{num}, broadcaster.PublishOwnerUpdate(ev.UserId, msg)
	}
}

func StreamUnlikeHandler(repo LikesStorer, broadcaster LikesBroadcaster) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.UnlikeEvent
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
		num, err := repo.Unlike(ev.TweetId, ev.UserId)

		reqBody := event.RequestBody{}
		_ = reqBody.FromUnlikeEvent(ev)
		msgBody := &event.Message_Body{}
		_ = msgBody.FromRequestBody(reqBody)
		msg := event.Message{
			Body:      msgBody,
			Path:      event.PRIVATE_POST_UNLIKE,
			Timestamp: time.Now(),
		}

		return event.LikesNumResponse{num}, broadcaster.PublishOwnerUpdate(ev.UserId, msg)
	}
}

func StreamGetLikesNumHandler(repo LikesStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetLikesNumEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}
		num, err := repo.LikesCount(ev.TweetId)
		if errors.Is(err, database.ErrLikesNotFound) {
			return event.LikesNumResponse{0}, nil
		}
		return event.LikesNumResponse{num}, err
	}
}

func StreamGetLikersHandler(likeRepo LikesStorer, userRepo LikedUserFetcher) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetLikersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.TweetId == "" {
			return nil, errors.New("empty tweet id")
		}
		likers, cur, err := likeRepo.Likers(ev.TweetId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}
		if len(likers) == 0 {
			return event.UsersResponse{
				Cursor: cur,
				Users:  []domain.User{},
			}, nil
		}

		users, err := userRepo.GetBatch(likers...)
		if err != nil {
			return nil, err
		}

		return event.UsersResponse{
			Cursor: cur,
			Users:  users,
		}, nil
	}
}
