package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"slices"
)

type OwnerUserStorer interface {
	GetOwner() (domain.Owner, error)
}

type UserFetcher interface {
	Get(userID string) (user domain.User, err error)
	ListRecommended(limit *uint64, cursor *string) ([]domain.User, string, error)
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

func StreamGetRecommendedUsersHandler(
	authRepo OwnerUserStorer,
	repo UserFetcher,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetAllUsersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		users, cursor, err := repo.ListRecommended(ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		owner, _ := authRepo.GetOwner()
		for i, user := range users {
			if owner.UserId == user.Id {
				slices.Delete(users, i, i+1)
				break
			}
		}

		return event.UsersResponse{
			Cursor: cursor,
			Users:  users,
		}, nil
	}
}
