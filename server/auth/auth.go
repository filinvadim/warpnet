package auth

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	event "github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/google/uuid"
	l "log"
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
		l.Println("already authenticated")
		return resp, errors.New("already authenticated")
	}
	l.Printf("authenticating user %s", message.Username)

	if err := as.userPersistence.Authenticate(message.Username, message.Password); err != nil {
		l.Printf("authentication failed: %v", err)
		return resp, fmt.Errorf("authentication failed: %v", err)
	}
	token := as.userPersistence.SessionToken()

	owner, err := as.userPersistence.GetOwner()
	if err != nil && !errors.Is(err, database.ErrOwnerNotFound) {
		l.Printf("owner fetching failed: %v", err)
		return resp, err
	}

	var user domain.User
	if errors.Is(err, database.ErrOwnerNotFound) {
		id := uuid.New().String()
		l.Println("creating new owner:", id)
		owner, err = as.userPersistence.SetOwner(domain.Owner{
			CreatedAt: time.Now(),
			Username:  message.Username,
			UserId:    id,
		})
		if err != nil {
			l.Printf("new owner creation failed: %v", err)
			return resp, fmt.Errorf("create owner: %v", err)

		}
		user, err = as.userPersistence.Create(domain.User{
			CreatedAt: owner.CreatedAt,
			Id:        id,
			NodeId:    "None",
			Username:  owner.Username,
		})
		if err != nil {
			return resp, fmt.Errorf("new user creation failed: %v", err)
		}
	}

	if owner.Username != message.Username {
		l.Printf("username mismatch: %s == %s", owner.Username, message.Username)
		return resp, fmt.Errorf("user %s doesn't exist", message.Username)
	}
	as.authReady <- domain.AuthNodeInfo{
		Identity: domain.Identity{Owner: owner, Token: token},
		Version:  "",
	}

	l.Println("OWNER USER ID:", owner.UserId)

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		l.Printf("node startup failed: timeout")
		return resp, errors.New("node starting is timed out")
	case nodeInfo := <-as.nodeReady:
		user.NodeId = nodeInfo.Identity.Owner.NodeId
		owner.NodeId = nodeInfo.Identity.Owner.NodeId
	}

	// update
	if owner, err = as.userPersistence.SetOwner(owner); err != nil {
		l.Printf("owner update failed: %v", err)
	}
	if _, err = as.userPersistence.Create(user); err != nil {
		l.Printf("user update failed: %v", err)
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
