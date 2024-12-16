package entity

import (
	"errors"
	"github.com/filinvadim/warpnet/chat/repository"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

// User represents the user of the application.
type User struct {
	ID             string    `json:"id,omitempty"`
	Name           string    `json:"name,omitempty"`
	Email          string    `json:"email,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	DeletedAt      time.Time `json:"deleted_at,omitempty"`
	HashedPassword string    `json:"hashed_password,omitempty"`
}

// NewUser returns a new user with the given name and email.
func NewUser(name, email string) *User {
	return &User{
		Name:  name,
		Email: email,
	}
}

func (u *User) Save(repo repository.User) error {
	var err error
	u.ID, err = repo.Create(u)
	return err
}
