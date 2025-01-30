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
	Create(user domain.User) (domain.User, error)
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
	userRepo UserStorer,
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

			if err := updateFollowingsNum(
				userRepo, ev.Followee, ev.Follower,
				func(existingNum int64) (newNum int64) {
					return existingNum + 1
				},
			); err != nil {
				log.Printf("error incrementing followers num: %v", err)
			}

			return nil, nil
		}
		mr.UnwrapStream(s, followF)
	}
}

func StreamUnfollowHandler(
	mr middleware.MiddlewareResolver,
	broadcaster FollowingBroadcaster,
	userRepo UserStorer,
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
			if err != nil {
				return nil, err
			}
			if err := updateFollowingsNum(
				userRepo, ev.Followee, ev.Follower,
				func(existingNum int64) (newNum int64) {
					return existingNum - 1
				},
			); err != nil {
				log.Printf("error decrementing followers num: %v", err)
			}
			return nil, nil
		}
		mr.UnwrapStream(s, unfollowF)
	}
}

func StreamGetFollowersHandler(
	mr middleware.MiddlewareResolver,
	userRepo UserStorer,
	repo FollowingStorer,
) func(s warpnet.WarpStream) {
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
			if err != nil {
				return nil, err
			}

			if err := updateFollowingsNum(
				userRepo, ev.UserId, "",
				func(_ int64) int64 {
					return int64(len(followers))
				},
			); err != nil {
				log.Printf("get followers: updating followers num: %v", err)
			}
			return event.FollowersResponse{
				Cursor:    cursor,
				Followee:  ev.UserId,
				Followers: followers,
			}, nil
		}
		mr.UnwrapStream(s, followersF)
	}
}

func StreamGetFolloweesHandler(
	mr middleware.MiddlewareResolver,
	userRepo UserStorer,
	repo FollowingStorer,
) func(s warpnet.WarpStream) {
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
			if err != nil {
				return nil, err
			}

			if err := updateFollowingsNum(
				userRepo, ev.UserId, "",
				func(_ int64) int64 {
					return int64(len(followees))
				},
			); err != nil {
				log.Printf("get followeEs: updating followers num: %v", err)
			}
			return event.FolloweesResponse{
				Cursor:    cursor,
				Follower:  ev.UserId,
				Followees: followees,
			}, nil
		}
		mr.UnwrapStream(s, followeesF)
	}
}

type followingNumUpdateClosure func(existingNum int64) (newNum int64)

func updateFollowingsNum(
	userRepo UserStorer,
	followeeId, followerId string,
	updateF followingNumUpdateClosure,
) error {
	if followeeId != "" {
		followeeUser, err := userRepo.Get(followeeId)
		if err != nil {
			return err
		}
		followeeUser.FollowersNum = updateF(followeeUser.FollowersNum)
		_, err = userRepo.Create(followeeUser)
		if err != nil {
			return err
		}
	}
	if followerId != "" {
		followerUser, err := userRepo.Get(followerId)
		if err != nil {
			return err
		}
		followerUser.FollowingNum = updateF(followerUser.FollowingNum)
		_, err = userRepo.Create(followerUser)
		return err
	}
	return nil
}
