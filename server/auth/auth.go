package auth

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"sync/atomic"
	"time"
)

type UserPersistencyLayer interface {
	Authenticate(username, password string) error
	SessionToken() string
	Create(user domain.User) (domain.User, error)
	GetOwner() domain.Owner
	SetOwner(domain.Owner) (domain.Owner, error)
}

type AuthService struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	interrupt       chan os.Signal
	nodeReady       chan domain.AuthNodeInfo
	authReady       chan domain.AuthNodeInfo
}

func NewAuthService(
	userPersistence UserPersistencyLayer,
	interrupt chan os.Signal,
	nodeReady chan domain.AuthNodeInfo,
	authReady chan domain.AuthNodeInfo,
) *AuthService {
	return &AuthService{
		new(atomic.Bool),
		userPersistence,
		interrupt,
		nodeReady,
		authReady,
	}
}

func (as *AuthService) IsAuthenticated() bool {
	return as.isAuthenticated.Load()
}

func (as *AuthService) AuthLogin(message event.LoginEvent) (resp event.LoginResponse, err error) {
	if as.isAuthenticated.Load() {
		token := as.userPersistence.SessionToken()
		owner := as.userPersistence.GetOwner()

		return event.LoginResponse{
			Token: token,
			Owner: owner,
		}, nil
	}
	log.Infof("authenticating user %s", message.Username)

	if err := as.userPersistence.Authenticate(message.Username, message.Password); err != nil {
		log.Errorf("authentication failed: %v", err)
		return resp, fmt.Errorf("authentication failed: %v", err)
	}
	token := as.userPersistence.SessionToken()
	owner := as.userPersistence.GetOwner()

	var user domain.User
	if owner.UserId == "" {
		id := uuid.New().String()
		log.Infoln("creating new owner:", id)
		owner, err = as.userPersistence.SetOwner(domain.Owner{
			CreatedAt: time.Now(),
			Username:  message.Username,
			UserId:    id,
		})
		if err != nil {
			log.Errorf("new owner creation failed: %v", err)
			return resp, fmt.Errorf("create owner: %v", err)

		}
		user, err = as.userPersistence.Create(domain.User{
			CreatedAt: owner.CreatedAt,
			Id:        id,
			NodeId:    "None",
			Username:  owner.Username,
			Rtt:       math.MaxInt64, // put your user at the end of a who-to-follow list
		})
		if err != nil {
			return resp, fmt.Errorf("new user creation failed: %v", err)
		}
	}

	if owner.Username != message.Username {
		log.Errorf("username mismatch: %s == %s", owner.Username, message.Username)
		return resp, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	as.authReady <- domain.AuthNodeInfo{
		Identity: domain.Identity{Owner: owner, Token: token},
	}

	log.Infoln("OWNER USER ID:", owner.UserId)

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		log.Errorln("node startup failed: timeout")
		return resp, errors.New("node starting is timed out")
	case nodeInfo := <-as.nodeReady:
		user.Id = owner.UserId
		user.Username = owner.Username
		user.CreatedAt = owner.CreatedAt
		user.Rtt = math.MaxInt64 // put your user at the end of a who-to-follow list

		user.NodeId = nodeInfo.Identity.Owner.NodeId
		owner.NodeId = nodeInfo.Identity.Owner.NodeId
	}

	if owner, err = as.userPersistence.SetOwner(owner); err != nil {
		log.Errorf("owner update failed: %v", err)
	}
	if _, err = as.userPersistence.Create(user); err != nil {
		log.Errorf("user update failed: %v", err)
	}

	as.isAuthenticated.Store(true)

	return event.LoginResponse{
		Token: token,
		Owner: owner,
	}, nil
}

func (as *AuthService) AuthLogout() error {
	as.interrupt <- os.Interrupt
	as.isAuthenticated.Store(false)
	return nil
}
