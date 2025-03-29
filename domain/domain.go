package domain

import (
	"github.com/filinvadim/warpnet/core/warpnet"
	"time"
)

// AuthNodeInfo defines model for AuthNodeInfo.
type AuthNodeInfo struct {
	Identity Identity         `json:"identity"`
	NodeInfo warpnet.NodeInfo `json:"node_info"`
}

// Chat defines model for Chat.
type Chat struct {
	CreatedAt   time.Time `json:"created_at"`
	Id          string    `json:"id"`
	OtherUserId string    `json:"other_user_id"`
	OwnerId     string    `json:"owner_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChatMessage defines model for ChatMessage.
type ChatMessage struct {
	ChatId      string    `json:"chat_id"`
	CreatedAt   time.Time `json:"created_at"`
	Id          string    `json:"id"`
	OtherUserId string    `json:"other_user_id"`
	OwnerId     string    `json:"owner_id"`
	Text        string    `json:"text"`
}

// Error defines model for Error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Following defines model for Following.
type Following struct {
	// Followee to user
	Followee string `json:"followee"`

	// Follower from user
	Follower         string  `json:"follower"`
	FollowerUsername *string `json:"follower_username,omitempty"`
}

// Identity defines model for Identity.
type Identity struct {
	Owner Owner  `json:"owner"`
	Token string `json:"token"`
}

// Like defines model for Like.
type Like struct {
	TweetId string `json:"tweet_id"`
	UserId  string `json:"user_id"`
}

// Owner defines model for Owner.
type Owner struct {
	CreatedAt time.Time `json:"created_at"`
	NodeId    string    `json:"node_id"`
	UserId    string    `json:"user_id"`
	Username  string    `json:"username"`
}

// ReplyNode defines model for ReplyNode.
type ReplyNode struct {
	Children []ReplyNode `json:"children"`
	Reply    Tweet       `json:"reply"`
}

const RetweetPrefix = "RT:"

// Tweet defines model for Tweet.
type Tweet struct {
	CreatedAt time.Time `json:"created_at"`
	Id        string    `json:"id"`
	ParentId  *string   `json:"parent_id,omitempty"`

	// RetweetedBy retweeted by user id
	RetweetedBy *string `json:"retweeted_by,omitempty"`
	RootId      string  `json:"root_id"`
	Text        string  `json:"text"`
	UserId      string  `json:"user_id"`
	Username    string  `json:"username"`
}

// User defines model for User.
type User struct {
	// Avatar mime type + "," + base64
	Avatar *string `json:"avatar,omitempty"`

	// BackgroundImage mime type + "," + base64
	BackgroundImage *string   `json:"background_image,omitempty"`
	Bio             string    `json:"bio"`
	Birthdate       string    `json:"birthdate"`
	CreatedAt       time.Time `json:"created_at"`
	FolloweesCount  uint64    `json:"followees_count"`
	FollowersCount  uint64    `json:"followers_count"`
	Id              string    `json:"id"`
	IsOffline       bool      `json:"isOffline"`
	NodeId          string    `json:"node_id"`

	// Rtt round trip time - nanoseconds, default - max int64
	Latency     int64   `json:"latency"`
	TweetsCount uint64  `json:"tweets_count"`
	Username    string  `json:"username"`
	Website     *string `json:"website,omitempty"`
}
