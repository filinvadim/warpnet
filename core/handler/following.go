package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
)

type UserStorer interface {
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
}

type FollowingBroadcaster interface {
	SubscribeUserUpdate(userId string) (err error)
	UnsubscribeUserUpdate(userId string) (err error)
}

type FollowingStorer interface {
	Follow(fromUserId, toUserId string) error
	Unfollow(fromUserId, toUserId string) error
	GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
	GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
}

func StreamFollowHandler(
	mr middleware.MiddlewareResolver,
	broadcaster FollowingBroadcaster,
	repo FollowingStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("follow handler:", err)
			return
		}

		followF := func(buf []byte) (any, error) {

			var ev event.NewFollowEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.Follower == "" || ev.Followee == "" {
				return nil, errors.New("empty follower or followee id")
			}

			if err := broadcaster.SubscribeUserUpdate(ev.Followee); err != nil {
				return nil, err
			}

			if err := repo.Follow(ev.Follower, ev.Followee); err != nil {
				_ = broadcaster.UnsubscribeUserUpdate(ev.Followee)
				return nil, err
			}
			return nil, err
		}
		mr.UnwrapStream(s, followF)
	}
}

func StreamUnfollowHandler(
	mr middleware.MiddlewareResolver,
	broadcaster FollowingBroadcaster,
	repo FollowingStorer,
) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		if err := mr.Authenticate(s); err != nil {
			log.Println("unfollow handler:", err)
			return
		}
		unfollowF := func(buf []byte) (any, error) {
			var ev event.NewUnfollowEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.Follower == "" || ev.Followee == "" {
				return nil, errors.New("empty follower or followee id")
			}

			if err := broadcaster.UnsubscribeUserUpdate(ev.Followee); err != nil {
				log.Println("unfollow unsubscribe:", err)
			}

			err = repo.Unfollow(ev.Follower, ev.Followee)
			return nil, err
		}
		mr.UnwrapStream(s, unfollowF)
	}
}

func StreamGetFollowersHandler(mr middleware.MiddlewareResolver, repo FollowingStorer) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		followersF := func(buf []byte) (any, error) {
			var ev event.GetFollowersEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.UserId == "" {
				return nil, errors.New("empty user id")
			}
			followers, cursor, err := repo.GetFollowers(ev.UserId, ev.Limit, ev.Cursor)
			return event.FollowersResponse{
				Cursor:    cursor,
				Followee:  ev.UserId,
				Followers: followers,
			}, err
		}
		mr.UnwrapStream(s, followersF)
	}
}

func StreamGetFolloweesHandler(mr middleware.MiddlewareResolver, repo FollowingStorer) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		followeesF := func(buf []byte) (any, error) {
			var ev event.GetFolloweesEvent
			err := json.JSON.Unmarshal(buf, &ev)
			if err != nil {
				return nil, err
			}
			if ev.UserId == "" {
				return nil, errors.New("empty user id")
			}
			followees, cursor, err := repo.GetFollowees(ev.UserId, ev.Limit, ev.Cursor)
			return event.FolloweesResponse{
				Cursor:    cursor,
				Follower:  ev.UserId,
				Followees: followees,
			}, err
		}
		mr.UnwrapStream(s, followeesF)
	}
}
