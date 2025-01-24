package handler

import (
	"errors"
	"fmt"
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

			fmt.Println("stream get user event:", ev.UserId)
			user, err := repo.Get(ev.UserId)
			if err != nil {
				users, _, _ := repo.List(nil, nil)
				for _, u := range users {
					fmt.Println("user:", u.Id)
				}
				return nil, err
			}

			if user.Id != "" {
				return user, nil
			}
			return nil, nil
		})
	}
}
