package storage

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode"
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
	for _, char := range userId {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return errors.New("userId must be alphanumeric")
		}
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

// ValidateTweetId ensures the tweet ID is valid (e.g., non-empty and numeric)
func validateTweetId(tweetId string) error {
	if len(tweetId) == 0 {
		return errors.New("tweetId cannot be empty")
	}
	for _, char := range tweetId {
		if !unicode.IsDigit(char) {
			return errors.New("tweetId must be numeric")
		}
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

// AddFolloweeId adds a followee ID segment to the key (reuses user ID validation)
func (pb *PrefixBuilder) AddFolloweeId(followeeId string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	// Perform validation and store the error if validation fails
	if err := validateUserId(followeeId); err != nil {
		pb.err = err
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, followeeId)
	return pb
}

// ValidateIPAddress ensures the IP address is valid (IPv4 format)
func validateIPAddress(ip string) error {
	if net.ParseIP(ip) == nil {
		return errors.New("invalid IP address format")
	}
	return nil
}

// AddIPAddress adds an IP address segment to the key after validation
func (pb *PrefixBuilder) AddIPAddress(ip string) *PrefixBuilder {
	// Skip processing if there's already an error
	if pb.err != nil {
		return pb
	}

	// Perform validation and store the error if validation fails
	if err := validateIPAddress(ip); err != nil {
		pb.err = err
		return pb
	}
	pb.key = fmt.Sprintf("%s:%s", pb.key, ip)
	return pb
}

// Build returns the final key string and any error that occurred during the chain
func (pb *PrefixBuilder) Build() (string, error) {
	return pb.key, pb.err
}
