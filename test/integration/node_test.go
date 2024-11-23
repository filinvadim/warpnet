package integration

import (
	"bytes"
	"encoding/json"
	"github.com/filinvadim/dWighter/config"
	"github.com/filinvadim/dWighter/domain-gen"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"github.com/oapi-codegen/runtime/types"
	"net/http"
	"testing"
	"time"
)

func TestNodeSimulator(t *testing.T) {
	for _, td := range testDataMap {
		testEvent := TestEvent{
			Timestamp: now.Format(time.RFC3339),
		}
		t.Log(td.eventType)
		time.Sleep(time.Second)

		testEvent.Data = td.data
		doRequest(td.eventType, testEvent, t)
		testEvent.Data = nil
	}
}

func doRequest(eventType node_gen.NewEventParamsEventType, data TestEvent, t *testing.T) {
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Send the HTTP request
	resp, err := http.Post(mainNodeURL+string(eventType), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to send POST request: %v", resp.Status)
	}
}

const (
	mainNodeURL  = "http://localhost:16969/v1/node/event/"
	testNodeHost = "node2"
	testUserId   = "node-2-test-user-id"
	testNodeId   = "node-2-test-node-id"
	testTweetId  = "node-2-test-tweet-id"
	testUsername = "TEST_NODE!"

	mainNodeUserId  = "e80c8856-462d-4eed-8052-4e49db8f73d1"
	mainNodeTweetId = "38966e99-9b62-4dd1-834f-c59330dd30b5"
)

func toStrPtr(s string) *string {
	return &s
}
func toIntPtr(i int64) *int64 {
	return &i
}

type TestEvent struct {
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

var now = time.Now()
var testUser = &domain.User{
	Birthdate:    &now,
	CreatedAt:    &now,
	Description:  toStrPtr("test node 2 description"),
	Followed:     &[]string{},
	FollowedNum:  toIntPtr(0),
	Followers:    &[]string{},
	FollowersNum: toIntPtr(0),
	Link:         toStrPtr("None"),
	Location:     nil,
	MyReferrals:  nil,
	NodeId:       types.UUID([]byte(testNodeId)),
	ReferredBy:   nil,
	UserId:       toStrPtr(testUserId),
	Username:     testUsername,
}

type testData struct {
	eventType node_gen.NewEventParamsEventType
	data      any
}

var testDataMap = []testData{
	{node_gen.NewUser, domain.NewUserEvent{User: testUser}},
	{node_gen.Ping, domain.PingEvent{
		CachedNodes: []domain.Node{},
		DestHost:    toStrPtr(config.InternalNodeAddress.String()),
		OwnerInfo:   testUser,
		OwnerNode: &domain.Node{
			CreatedAt: &now,
			Host:      testNodeHost,
			Id:        types.UUID([]byte(testNodeId)),
			IsActive:  true,
			IsOwned:   true,
			LastSeen:  now,
			Latency:   toIntPtr(69),
			OwnerId:   testUserId,
			Uptime:    toIntPtr(69),
		},
	}},
	{node_gen.NewTweet, domain.NewTweetEvent{
		Tweet: &domain.Tweet{
			Content:       "TEST TWEET",
			CreatedAt:     &now,
			Likes:         nil,
			LikesCount:    toIntPtr(0),
			Retweets:      nil,
			RetweetsCount: nil,
			Sequence:      toIntPtr(2),
			TweetId:       toStrPtr(testTweetId),
			UserId:        testUserId,
			Username:      toStrPtr(testUsername),
		},
	}},
	{node_gen.GetTweet, domain.GetTweetEvent{
		TweetId: mainNodeTweetId,
		UserId:  mainNodeUserId,
	}},
	{node_gen.GetTweets, domain.GetAllTweetsEvent{
		Cursor: nil,
		Limit:  nil,
		UserId: mainNodeUserId,
	}},
	{node_gen.GetUser, domain.GetUserEvent{UserId: mainNodeUserId}},
	{node_gen.GetTimeline, domain.GetTimelineEvent{
		Cursor: nil,
		Limit:  nil,
		UserId: mainNodeUserId,
	}},
}
