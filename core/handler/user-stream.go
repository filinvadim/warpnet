package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
)

func StreamGetUserHandler(repo *database.UserRepo) func(s network.Stream) {
	return func(s network.Stream) {
		ReadStream(s, func(buf []byte) (any, error) {
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
		})
	}
}
