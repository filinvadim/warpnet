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

const (
	mainNodeURL  = "http://localhost:16969/v1/node/event/"
	testNodeHost = "node3"
	testId       = "node-3-test-user-id"
	testNodeId   = "node-3-test-node-id"
	testTweetId  = "node-3-test-tweet-id"
	testUsername = "TEST_NODE3!"

	mainNodeId      = "e80c8856-462d-4eed-8052-4e49db8f73d1"
	mainNodeTweetId = "38966e99-9b62-4dd1-834f-c59330dd30b5"
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

	resp, err := http.Post(mainNodeURL+string(eventType), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to send POST request: %v", resp.Status)
	}
}

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
	Birthdate:   &now,
	CreatedAt:   now,
	Description: ("test node 2 description"),
	Link:        toStrPtr("None"),
	Location:    nil,
	MyReferrals: nil,
	NodeId:      types.UUID([]byte(testNodeId)),
	ReferredBy:  nil,
	Id:          (testId),
	Username:    testUsername,
}

type testData struct {
	eventType node_gen.NewEventParamsEventType
	data      any
}

var testDataMap = []testData{
	{node_gen.NewUser, domain.NewUserEvent{User: testUser}},
	{node_gen.Ping, domain.PingEvent{
		Nodes:     []domain.Node{},
		DestHost:  toStrPtr(config.InternalNodeAddress.String()),
		OwnerInfo: testUser,
		OwnerNode: &domain.Node{
			CreatedAt: now,
			Host:      testNodeHost,
			Id:        types.UUID([]byte(testNodeId)),
			IsActive:  true,
			IsOwned:   true,
			LastSeen:  now,
			Latency:   toIntPtr(69),
			OwnerId:   testId,
			Uptime:    toIntPtr(69),
		},
	}},
	{node_gen.NewTweet, domain.NewTweetEvent{
		Tweet: &domain.Tweet{
			Content:   "TEST TWEET",
			CreatedAt: now,
			Id:        (testTweetId),
			UserId:    testId,
			Username:  (testUsername),
		},
	}},
	{node_gen.GetTweet, domain.GetTweetEvent{
		TweetId: testTweetId,
		UserId:  testId,
	}},
	{node_gen.GetTweets, domain.GetAllTweetsEvent{
		UserId: testId,
	}},
	{node_gen.GetUser, domain.GetUserEvent{UserId: testId}},
	{node_gen.GetTimeline, domain.GetTimelineEvent{
		UserId: testId,
	}},
}
