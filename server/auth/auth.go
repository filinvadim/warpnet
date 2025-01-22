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
	Owner() (domain.User, error)
	CreateOwner(o domain.User) (err error)
	GetOwner() (domain.Owner, error)
	SetOwner(domain.Owner) (domain.Owner, error)
}

type AuthService struct {
	isAuthenticated *atomic.Bool
	userPersistence UserPersistencyLayer
	interrupt       chan os.Signal
	nodeReady       chan string
	authReady       chan struct{}
}

func NewAuthService(
	userPersistence UserPersistencyLayer,
	interrupt chan os.Signal,
	nodeReady chan string,
	authReady chan struct{},
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

	owner, err := as.userPersistence.Owner()
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		l.Errorf("owner fetching failed: %v", err)
		return resp, err
	}
	if errors.Is(err, database.ErrUserNotFound) {
		l.Info("creating new owner")
		err := as.userPersistence.CreateOwner(domain.User{
			CreatedAt: time.Now(),
			Username:  message.Username,
		})
		if err != nil {
			l.Errorf("new owner creation failed: %v", err)
			return resp, fmt.Errorf("create owner: %v", err)

		}
		owner, _ = as.userPersistence.Owner()
	}

	if owner.Username != message.Username {
		l.Errorf("username mismatch: %s == %s", owner.Username, message.Username)
		return resp, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	as.authReady <- struct{}{}

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		l.Errorf("node startup failed: timeout")
		return resp, errors.New("node starting is timed out")
	case nodeId := <-as.nodeReady:
		owner.NodeId = nodeId
	}

	if err = as.userPersistence.CreateOwner(owner); err != nil {
		l.Errorf("owner update failed: %v", err)
		return resp, fmt.Errorf("failed to update user: %v", err)
	}

	as.isAuthenticated.Store(true)

	data := api.LoginData{
		Token: token,
		User:  owner,
	}

	return api.LoginResponse{Data: &data}, nil
}

func (as *AuthService) AuthLogout() error {
	as.interrupt <- os.Interrupt
	as.isAuthenticated.Store(false)
	return nil
}
