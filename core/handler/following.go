package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type FollowNodeStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error)
}

type FollowingAuthStorer interface {
	GetOwner() (domain.Owner, error)
}

type FollowingUserStorer interface {
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
	Create(user domain.User) (domain.User, error)
}

type FollowingBroadcaster interface {
	SubscribeUserUpdate(userId string) (err error)
	UnsubscribeUserUpdate(userId string) (err error)
}

type FollowingStorer interface {
	Follow(fromUserId, toUserId string, event domain.Following) error
	Unfollow(fromUserId, toUserId string) error
	GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
	GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
}

func StreamFollowHandler(
	broadcaster FollowingBroadcaster,
	followRepo FollowingStorer,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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

		if err := followRepo.Follow(ev.Follower, ev.Followee, domain.Following{
			Followee:         ev.Followee,
			Follower:         ev.Follower,
			FollowerUsername: ev.FollowerUsername,
		}); err != nil {
			_ = broadcaster.UnsubscribeUserUpdate(ev.Followee)
			return nil, err
		}
		return nil, nil
	}
}

func StreamUnfollowHandler(
	broadcaster FollowingBroadcaster,
	followRepo FollowingStorer,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.NewUnfollowEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.Follower == "" || ev.Followee == "" {
			return nil, errors.New("empty follower or followee id")
		}

		if err := broadcaster.UnsubscribeUserUpdate(ev.Followee); err != nil {
			log.Infoln("unfollow unsubscribe:", err)
		}

		err = followRepo.Unfollow(ev.Follower, ev.Followee)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func StreamGetFollowersHandler(
	authRepo *database.AuthRepo,
	userRepo FollowingUserStorer,
	followRepo FollowingStorer,
	streamer FollowNodeStreamer,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetFollowersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		owner, _ := authRepo.GetOwner()
		if ev.UserId != owner.UserId { // redirect
			user, err := userRepo.Get(ev.UserId)
			if err != nil {
				return nil, err
			}
			return streamer.GenericStream(user.NodeId, event.PUBLIC_GET_FOLLOWERS_1_0_0, buf)
		}

		followers, cursor, err := followRepo.GetFollowers(ev.UserId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		return event.FollowersResponse{
			Cursor:    cursor,
			Followee:  ev.UserId,
			Followers: followers,
		}, nil
	}
}

func StreamGetFolloweesHandler(
	authRepo *database.AuthRepo,
	userRepo FollowingUserStorer,
	followRepo FollowingStorer,
	streamer FollowNodeStreamer,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetFolloweesEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		owner, _ := authRepo.GetOwner()
		if ev.UserId != owner.UserId { // redirect
			user, err := userRepo.Get(ev.UserId)
			if err != nil {
				return nil, err
			}
			return streamer.GenericStream(user.NodeId, event.PUBLIC_GET_FOLLOWEES_1_0_0, buf)
		}

		followees, cursor, err := followRepo.GetFollowees(ev.UserId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		return event.FolloweesResponse{
			Cursor:    cursor,
			Follower:  ev.UserId,
			Followees: followees,
		}, nil
	}
}
