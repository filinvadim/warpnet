// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type UserStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
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
	Get(userId string) (user domain.User, err error)
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
	authRepo UserAuthStorer,
	streamer UserStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetUserEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, fmt.Errorf("get user: event unmarshal: %v %s", err, buf)
		}

		if ev.UserId == "" {
			return nil, warpnet.WarpError("empty user id")
		}

		ownerId := authRepo.GetOwner().UserId

		var u domain.User
		if ev.UserId == ownerId {
			u, err = repo.Get(ownerId)
			if err != nil {
				return nil, err
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

		otherUser, err := repo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		otherUserData, err := streamer.GenericStream(
			otherUser.NodeId,
			event.PUBLIC_GET_USER,
			ev,
		)
		if errors.Is(err, warpnet.ErrNodeIsOffline) {
			u, err = repo.Get(otherUser.Id)
			if err != nil {
				return nil, err
			}
			u.IsOffline = true
			return u, nil
		}
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(otherUserData, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other user error response: %s", possibleError.Message)
		}

		if err = json.JSON.Unmarshal(otherUserData, &u); err != nil {
			return nil, fmt.Errorf("get other user: response unmarshal: %v %s", err, otherUserData)
		}
		_, err = repo.Update(u.Id, u)
		return u, err
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

		if ev.UserId == "" {
			return nil, warpnet.WarpError("empty user id")
		}

		ownerId := authRepo.GetOwner().UserId

		if ev.UserId == ownerId {
			users, cursor, err := userRepo.List(ev.Limit, ev.Cursor)
			if err != nil {
				return nil, err
			}

			return event.UsersResponse{
				Cursor: cursor,
				Users:  users,
			}, nil
		}

		otherUser, err := userRepo.Get(ev.UserId)
		if err != nil {
			return nil, fmt.Errorf("other user get: %v", err)
		}

		usersDataResp, err := streamer.GenericStream(
			otherUser.NodeId,
			event.PUBLIC_GET_USERS,
			ev,
		)
		if err != nil {
			return event.UsersResponse{
				Cursor: "",
				Users:  []domain.User{},
			}, nil
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(usersDataResp, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other users error response: %s", possibleError.Message)
		}

		var usersResp event.UsersResponse
		if err := json.JSON.Unmarshal(usersDataResp, &usersResp); err != nil {
			return nil, err
		}

		for _, user := range usersResp.Users {
			_, err := userRepo.Create(user)
			if errors.Is(err, database.ErrUserAlreadyExists) {
				_, _ = userRepo.Update(user.Id, user)
				continue
			}
			if err != nil {
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
