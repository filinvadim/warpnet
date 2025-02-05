package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
)

type OwnerUserStorer interface {
	GetOwner() (domain.Owner, error)
}

type UserFetcher interface {
	Get(userID string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
}

type NearestPeersSeeker interface {
	GetNearestPeers(limit *uint64, cursor *string) (_ []warpnet.WarpPeerID, cur string, err error)
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
	userRepo UserFetcher,
	nodeRepo NearestPeersSeeker,
) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.GetRecommendedUsersEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		nearLimitDefault := uint64(100)
		nearest, recCursor, err := nodeRepo.GetNearestPeers(&nearLimitDefault, ev.RecommendsCursor)
		if err != nil {
			return nil, err
		}

		users, usersCursor, err := userRepo.List(ev.Limit, ev.UsersCursor)
		if err != nil {
			return nil, err
		}

		owner, _ := authRepo.GetOwner()

		var (
			recommended        = make([]domain.User, 0, len(users))
			limit       uint64 = 20
			userMap            = make(map[string]domain.User, len(users))
			ownPeerID          = warpnet.WarpPeerID(owner.NodeId)
		)
		if ev.Limit != nil {
			limit = *ev.Limit
		}

		for _, user := range users {
			if owner.UserId == user.Id {
				continue
			}
			if user.Username == "" || user.Id == "" {
				continue
			}
			userMap[user.NodeId] = user
		}

		for _, pId := range nearest {
			if ownPeerID == pId {
				continue
			}
			user, ok := userMap[pId.String()]
			if !ok {
				continue
			}
			recommended = append(recommended, user)
			if len(recommended) == int(limit) {
				recCursor = ""
				usersCursor = ""
				break
			}
		}

		return event.RecommendedResponse{
			RecommendsCursor: recCursor,
			UsersCursor:      usersCursor,
			Users:            recommended,
		}, nil
	}
}
