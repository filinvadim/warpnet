package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
)

type UserFetcher interface {
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
	Update(userId string, newUser domain.User) (updatedUser domain.User, err error)
}

type UserAuthStorer interface {
	GetOwner() domain.Owner
}

func StreamGetUserHandler(repo UserFetcher) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetUserEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user id")
		}

		user, err := repo.Get(ev.UserId)
		if err != nil {
			return nil, err
		}

		if user.Id == "" {
			return event.ErrorResponse{
				Code:    404,
				Message: "user not found",
			}, nil
		}

		return user, nil
	}
}

func StreamGetUsersHandler(
	userRepo UserFetcher,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetAllUsersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		users, cursor, err := userRepo.List(ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		return event.UsersResponse{
			Cursor: cursor,
			Users:  users,
		}, nil
	}
}

func StreamUpdateProfileHandler(authRepo UserAuthStorer, userRepo UserFetcher) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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
