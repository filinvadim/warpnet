package chat

import (
	"sync"
)

// TableCache represents a table with distributed cache.
type ChatCache struct {
	mxUser     sync.RWMutex
	userToRoom map[string]string
	mxRoom     sync.RWMutex
	roomToUser map[string]string
}

// NewTableCache returns a new TableCache.
func NewTableCache() *ChatCache {
	return &ChatCache{}
}

// Add adds both user and room to the cache.
func (t *ChatCache) Add(user, room string) (err error) {
	//pipe := t.client.Pipeline()
	//// Add room to user's list.
	//pipe.SAdd(userKey(user), room)
	//// Add user to the room.
	//pipe.SAdd(roomKey(room), user)
	//_, err := pipe.Exec()
	return err
}

// Delete removes the user from the room, and clears the user's room list.
func (t *ChatCache) Delete(user string, fn func(string)) (err error) {
	//// Get the rooms the user is in.
	//rooms := t.client.SMembers(userKey(user)).Val()
	//
	//pipe := t.client.Pipeline()
	//// For each room, remove the user from the set.
	//for _, room := range rooms {
	//	pipe.SRem(roomKey(room), user)
	//	fn(room)
	//}
	//_, err := pipe.Exec()
	return err
}

// GetRooms returns the rooms the user is in.
func (t *ChatCache) GetRooms(user string) []string {
	//return t.client.SMembers(userKey(user)).Val()
	return nil
}

// GetUsers returns the users in a room.
func (t *ChatCache) GetUsers(room string) []string {
	//return t.client.SMembers(roomKey(room)).Val()
	return nil
}
