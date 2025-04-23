package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type FollowNodeStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error)
}

type FollowingAuthStorer interface {
	GetOwner() domain.Owner
}

type FollowingUserStorer interface {
	Get(userId string) (user domain.User, err error)
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
	authRepo FollowingAuthStorer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewFollowEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.Follower == "" || ev.Followee == "" {
			return nil, errors.New("empty follower or followee id")
		}

		isImFollowing := authRepo.GetOwner().UserId == ev.Follower

		if err := followRepo.Follow(ev.Follower, ev.Followee, domain.Following{
			Followee:         ev.Followee,
			Follower:         ev.Follower,
			FollowerUsername: ev.FollowerUsername,
		}); err != nil {
			return nil, err
		}

		if isImFollowing {
			if err := broadcaster.SubscribeUserUpdate(ev.Followee); err != nil {
				return nil, err
			}
		}

		return event.Accepted, nil
	}
}

func StreamUnfollowHandler(
	broadcaster FollowingBroadcaster,
	followRepo FollowingStorer,
	authRepo FollowingAuthStorer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewUnfollowEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.Follower == "" || ev.Followee == "" {
			return nil, errors.New("empty follower or followee id")
		}

		isImUnfollowing := authRepo.GetOwner().UserId == ev.Follower

		err = followRepo.Unfollow(ev.Follower, ev.Followee)
		if err != nil {
			return nil, err
		}

		if isImUnfollowing {
			if err := broadcaster.UnsubscribeUserUpdate(ev.Followee); err != nil {
				log.Infoln("unfollow unsubscribe:", err)
			}
		}

		return event.Accepted, nil
	}
}

func StreamGetFollowersHandler(
	authRepo FollowingAuthStorer,
	userRepo FollowingUserStorer,
	followRepo FollowingStorer,
	streamer FollowNodeStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetFollowersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		owner := authRepo.GetOwner()
		if ev.UserId == owner.UserId {
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

		user, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}
		followersData, err := streamer.GenericStream(user.NodeId, event.PUBLIC_GET_FOLLOWERS, buf)
		if errors.Is(err, warpnet.ErrNodeIsOffline) {
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
		if err != nil {
			return nil, err
		}
		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(followersData, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other followers error response: %s", possibleError.Message)
		}

		var followersResp event.FollowersResponse
		if err := json.JSON.Unmarshal(followersData, &followersResp); err != nil {
			return nil, err
		}
		return followersResp, nil
	}
}

func StreamGetFolloweesHandler(
	authRepo FollowingAuthStorer,
	userRepo FollowingUserStorer,
	followRepo FollowingStorer,
	streamer FollowNodeStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetFolloweesEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}
		owner := authRepo.GetOwner()
		if ev.UserId == owner.UserId {
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

		user, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}
		followeesData, err := streamer.GenericStream(user.NodeId, event.PUBLIC_GET_FOLLOWEES, buf)
		if errors.Is(err, warpnet.ErrNodeIsOffline) {
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
		if err != nil {
			return nil, err
		}
		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(followeesData, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other followees error response: %s", possibleError.Message)
		}

		var followeesResp event.FolloweesResponse
		if err := json.JSON.Unmarshal(followeesData, &followeesResp); err != nil {
			return nil, err
		}
		return followeesResp, nil
	}
}
