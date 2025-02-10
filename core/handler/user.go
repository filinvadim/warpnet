package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type UserStreamer interface {
	GenericStream(nodeId warpnet.WarpPeerID, path stream.WarpRoute, data any) (_ []byte, err error)
}

type UserTweetsCounter interface {
	TweetsCount(userID string) (uint64, error)
}

type UserFollowsCounter interface {
	GetFollowersCount(userId string) (uint64, error)
	GetFolloweesCount(userId string) (uint64, error)
}

type UserFetcher interface {
	Create(user domain.User) (domain.User, error)
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
	Update(userId string, newUser domain.User) (updatedUser domain.User, err error)
}

type UserAuthStorer interface {
	GetOwner() domain.Owner
}

func StreamGetUserHandler(
	tweetRepo UserTweetsCounter,
	followRepo UserFollowsCounter,
	repo UserFetcher,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetUserEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}

		u, err := repo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		if u.Id == "" {
			return event.ErrorResponse{
				Code:    404,
				Message: "user not found",
			}, nil
		}

		followersCount, err := followRepo.GetFollowersCount(u.Id)
		if err != nil {
			return nil, err
		}
		followeesCount, err := followRepo.GetFolloweesCount(u.Id)
		if err != nil {
			return nil, err
		}
		tweetsCount, err := tweetRepo.TweetsCount(u.Id)
		if err != nil {
			return nil, err
		}

		u.TweetsCount = tweetsCount
		u.FollowersCount = followersCount
		u.FolloweesCount = followeesCount

		return u, nil
	}
}

func StreamGetUsersHandler(
	userRepo UserFetcher,
	authRepo UserAuthStorer,
	streamer UserStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllUsersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		if ev.UserId == nil {
			users, cursor, err := userRepo.List(ev.Limit, ev.Cursor)
			if err != nil {
				return nil, err
			}

			return event.UsersResponse{
				Cursor: cursor,
				Users:  users,
			}, nil
		}

		otherUser, err := userRepo.Get(*ev.UserId)
		if err != nil {
			return nil, err
		}

		usersDataResp, err := streamer.GenericStream(
			warpnet.WarpPeerID(otherUser.NodeId),
			event.PUBLIC_GET_USERS,
			ev,
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(usersDataResp, &possibleError); err == nil {
			return nil, fmt.Errorf("create other chat stream: %s", possibleError.Message)
		}

		var usersResp event.UsersResponse
		if err := json.JSON.Unmarshal(usersDataResp, &usersResp); err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()

		for _, user := range usersResp.Users {
			if user.Id == owner.UserId {
				continue
			}
			if _, err := userRepo.Create(user); err != nil {
				return nil, err
			}
		}
		return usersResp, nil
	}
}

func StreamUpdateProfileHandler(authRepo UserAuthStorer, userRepo UserFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewUserEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		owner := authRepo.GetOwner()

		updatedUser, err := userRepo.Update(owner.UserId, ev)
		if err != nil {
			log.Errorln("failed to update user data", err)
			return nil, err
		}
		return updatedUser, nil
	}
}
