package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
)

type UserFetcher interface {
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
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

		if user.Id != "" {
			return user, nil
		}
		return nil, nil
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
