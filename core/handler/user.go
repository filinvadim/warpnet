package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	warpnet "github.com/filinvadim/warpnet/core/warpnet"
	domain "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
)

type UserFetcher interface {
	Get(userID string) (user domain.User, err error)
}

func StreamGetUserHandler(mr middleware.MiddlewareResolver, repo UserFetcher) func(s warpnet.WarpStream) {
	return func(s warpnet.WarpStream) {
		getUserF := func(buf []byte) (any, error) {
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
		mr.UnwrapStream(s, getUserF)
	}
}
