package auth

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"sync/atomic"
	"time"
)

type UserPersistencyLayer interface {
	Create(user domain.User) (domain.User, error)
	Update(userId string, newUser domain.User) (domain.User, error)
}

type AuthPersistencyLayer interface {
	Authenticate(username, password string) error
	SessionToken() string
	GetOwner() domain.Owner
	SetOwner(domain.Owner) (domain.Owner, error)
}

type AuthService struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	authPersistence AuthPersistencyLayer
	interrupt       chan os.Signal
	authReady       chan domain.AuthNodeInfo
}

func NewAuthService(
	authRepo AuthPersistencyLayer,
	userRepo UserPersistencyLayer,
	interrupt chan os.Signal,
	authReady chan domain.AuthNodeInfo,
) *AuthService {
	return &AuthService{
		new(atomic.Bool),
		userRepo,
		authRepo,
		interrupt,
		authReady,
	}
}

func (as *AuthService) IsAuthenticated() bool {
	return as.isAuthenticated.Load()
}

func (as *AuthService) AuthLogin(message event.LoginEvent) (authInfo event.LoginResponse, err error) {
	if as.isAuthenticated.Load() {
		return event.LoginResponse{
			Identity: domain.Identity{
				Token: as.authPersistence.SessionToken(),
				Owner: as.authPersistence.GetOwner(),
			},
		}, nil
	}
	log.Infof("authenticating user %s", message.Username)

	if err := as.authPersistence.Authenticate(message.Username, message.Password); err != nil {
		log.Errorf("authentication failed: %v", err)
		return authInfo, fmt.Errorf("authentication failed: %v", err)
	}
	token := as.authPersistence.SessionToken()
	owner := as.authPersistence.GetOwner()

	var user domain.User
	if owner.UserId == "" {
		id := uuid.New().String()
		log.Infoln("creating new owner:", id)
		owner, err = as.authPersistence.SetOwner(domain.Owner{
			CreatedAt: time.Now(),
			Username:  message.Username,
			UserId:    id,
		})
		if err != nil {
			log.Errorf("new owner creation failed: %v", err)
			return authInfo, fmt.Errorf("create owner: %v", err)

		}
		user, err = as.userPersistence.Create(domain.User{
			CreatedAt: owner.CreatedAt,
			Id:        id,
			NodeId:    "None",
			Username:  owner.Username,
			Rtt:       math.MaxInt64, // put your user at the end of a who-to-follow list
		})
		if err != nil {
			return authInfo, fmt.Errorf("new user creation failed: %v", err)
		}
	}

	if owner.Username != message.Username {
		log.Errorf("username mismatch: %s == %s", owner.Username, message.Username)
		return authInfo, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	as.authReady <- domain.AuthNodeInfo{
		Identity: domain.Identity{Owner: owner, Token: token},
	}

	log.Infoln("OWNER USER ID:", owner.UserId)

	timer := time.NewTimer(time.Minute * 5)
	defer timer.Stop()
	select {
	case <-timer.C:
		log.Errorln("node startup failed: timeout")
		return authInfo, errors.New("node starting is timed out")
	case authInfo = <-as.authReady:
		user.Id = owner.UserId
		user.Username = owner.Username
		user.CreatedAt = owner.CreatedAt
		user.Rtt = math.MaxInt64 // put your user at the end of a who-to-follow list
		user.NodeId = authInfo.Identity.Owner.NodeId
		owner.NodeId = authInfo.Identity.Owner.NodeId
	}

	if owner, err = as.authPersistence.SetOwner(owner); err != nil {
		log.Errorf("owner update failed: %v", err)
	}
	if _, err = as.userPersistence.Update(user.Id, user); err != nil {
		log.Errorf("user update failed: %v", err)
	}

	as.isAuthenticated.Store(true)

	return event.LoginResponse(authInfo), nil
}

func (as *AuthService) AuthLogout() error {
	as.interrupt <- os.Interrupt
	as.isAuthenticated.Store(false)
	return nil
}
