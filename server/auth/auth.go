package auth

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	domain "github.com/filinvadim/warpnet/gen/domain-gen"
	event "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	"os"
	"sync/atomic"
	"time"
)

type UserPersistencyLayer interface {
	Authenticate(username, password string) error
	SessionToken() string
	Create(user domain.User) (domain.User, error)
	GetOwner() (domain.Owner, error)
	SetOwner(domain.Owner) (domain.Owner, error)
}

type AuthService struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	interrupt       chan os.Signal
	nodeReady       chan domain.Owner
	authReady       chan domain.Owner
}

func NewAuthService(
	userPersistence UserPersistencyLayer,
	interrupt chan os.Signal,
	nodeReady chan domain.Owner,
	authReady chan domain.Owner,
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

func (as *AuthService) AuthLogin(l echo.Logger, message event.LoginEvent) (resp api.LoginResponse, err error) {
	if as.isAuthenticated.Load() {
		l.Warn("already authenticated")
		return resp, errors.New("already authenticated")
	}
	l.Infof("authenticating user %s", message.Username)

	if err := as.userPersistence.Authenticate(message.Username, message.Password); err != nil {
		l.Errorf("authentication failed: %v", err)
		return resp, fmt.Errorf("authentication failed: %v", err)
	}
	token := as.userPersistence.SessionToken()

	owner, err := as.userPersistence.GetOwner()
	if err != nil && !errors.Is(err, database.ErrOwnerNotFound) {
		l.Errorf("owner fetching failed: %v", err)
		return resp, err
	}

	var user domain.User
	if errors.Is(err, database.ErrOwnerNotFound) {
		l.Info("creating new owner")
		owner, err = as.userPersistence.SetOwner(domain.Owner{
			CreatedAt: time.Now(),
			Username:  message.Username,
		})
		if err != nil {
			l.Errorf("new owner creation failed: %v", err)
			return resp, fmt.Errorf("create owner: %v", err)

		}
		user, err = as.userPersistence.Create(domain.User{
			CreatedAt: owner.CreatedAt,
			Id:        owner.UserId,
			NodeId:    "",
			Username:  owner.Username,
		})
		if err != nil {
			return resp, fmt.Errorf("new user creation failed: %v", err)
		}
	}

	if owner.Username != message.Username {
		l.Errorf("username mismatch: %s == %s", owner.Username, message.Username)
		return resp, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	as.authReady <- owner

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		l.Errorf("node startup failed: timeout")
		return resp, errors.New("node starting is timed out")
	case owner = <-as.nodeReady:
		user.NodeId = owner.NodeId
	}

	// update
	if _, err = as.userPersistence.SetOwner(owner); err != nil {
		l.Errorf("owner update failed: %v", err)
	}
	if _, err = as.userPersistence.Create(user); err != nil {
		l.Errorf("user update failed: %v", err)
	}

	as.isAuthenticated.Store(true)

	data := api.LoginData{
		Token: token,
		User:  owner,
	}

	return api.LoginResponse{Data: data}, nil
}

func (as *AuthService) AuthLogout() error {
	as.interrupt <- os.Interrupt
	as.isAuthenticated.Store(false)
	return nil
}
