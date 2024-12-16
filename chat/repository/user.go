package repository

import "github.com/filinvadim/warpnet/chat/entity"

// User represents the DAO for user in the database.
type User interface {
	CreateUser(user *entity.User) error
	GetUser(id string) (entity.User, error)
	GetUserByName(name string) (entity.User, error)
	GetUserByEmail(email string) (entity.User, error)
	GetUsers(id int) ([]entity.Friend, error)
}
