package storage

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PrefixBuilder is a struct that holds a key and any potential error
type PrefixBuilder struct {
	key string
	err error
}

// NewPrefixBuilder creates a new PrefixBuilder instance
func NewPrefixBuilder(base string) *PrefixBuilder {
	return &PrefixBuilder{key: base}
}

// AddPrefix adds a prefix to the key if it does not already exist
func (pb *PrefixBuilder) AddPrefix(prefix string) *PrefixBuilder {
	// If there's already an error, skip further processing
	if pb.err != nil {
		return pb
	}

	if !strings.HasPrefix(pb.key, prefix) {
		pb.key = fmt.Sprintf("%s:%s", pb.key, prefix)
	}
	return pb
}

// ValidateUserId ensures the user ID is valid (e.g., non-empty and alphanumeric)
func validateUserId(userId string) error {
	if len(userId) == 0 {
		return errors.New("userId cannot be empty")
	}

	return nil
}

// AddUserId adds a user ID segment to the key after validation
func (pb *PrefixBuilder) AddUserId(userId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	// Perform validation and store the error if validation fails
	if err := validateUserId(userId); err != nil {
		pb.err = err
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, userId)
	return pb
}

func (pb *PrefixBuilder) AddNodeId(nodeId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	if err := uuid.Validate(nodeId); err != nil {
		pb.err = err
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, nodeId)
	return pb
}

// AddUserId adds a user ID segment to the key after validation
func (pb *PrefixBuilder) AddUsername(username string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	// Perform validation and store the error if validation fails
	if username == "" {
		pb.err = errors.New("username cannot be empty")
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, username)
	return pb
}

func (pb *PrefixBuilder) AddReverseTimestamp(tm time.Time) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:%s", pb.key, strconv.FormatInt(math.MaxInt64-tm.UnixMilli(), 10))
	return pb
}

func (pb *PrefixBuilder) AddSequence(num int64) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:%s", pb.key, strconv.FormatInt(num, 10))
	return pb
}

// ValidateTweetId ensures the tweet ID is valid (e.g., non-empty and numeric)
func validateTweetId(tweetId string) error {
	if len(tweetId) == 0 {
		return errors.New("tweetId cannot be empty")
	}

	return nil
}

// AddTweetId adds a tweet ID segment to the key after validation
func (pb *PrefixBuilder) AddTweetId(tweetId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	// Perform validation and store the error if validation fails
	if err := validateTweetId(tweetId); err != nil {
		pb.err = err
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, tweetId)
	return pb
}

// AddFollowedId adds a followee ID segment to the key (reuses user ID validation)
func (pb *PrefixBuilder) AddWriterId(writerId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:writer:%s", pb.key, writerId)
	return pb
}

func (pb *PrefixBuilder) AddReaderId(readerId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:reader:%s", pb.key, readerId)
	return pb
}

// AddIPAddress adds an IP address segment to the key after validation
func (pb *PrefixBuilder) AddHostAddress(host string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:%s", pb.key, host)
	return pb
}

func (pb *PrefixBuilder) AddSettingName(name string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	pb.key = fmt.Sprintf("%s:%s", pb.key, name)
	return pb
}

// Build returns the final key string and any error that occurred during the chain
func (pb *PrefixBuilder) Build() (string, error) {
	return pb.key, pb.err
}
