package storage

import (
	"fmt"
	"math"
	"strings"
)

const (
	SortableDelimiter = ":"
)

type (
	Namespace                 string
	NamespaceUserId           string
	NamespaceUserIdSeqTweetId string
	NamespaceNodeId           string
	NamespaceNodeIdHost       string
	DatabaseKey               string
)

// PrefixBuilder is a struct that holds a key and any potential error
type PrefixBuilder struct {
	namespace Namespace
}

// NewPrefixBuilder creates a new PrefixBuilder instance
func NewPrefixBuilder(ns string) Namespace {
	return Namespace(ns)
}

// AddPrefix adds a prefix to the key if it does not already exist
func (ns Namespace) AddCustomPrefix(prefix string) NamespaceUserId {
	key := fmt.Sprintf("%s:%s", ns, prefix)
	return NamespaceUserId(key)
}

func (ns Namespace) Build() DatabaseKey {
	key := string(ns)
	if !strings.Contains(key, SortableDelimiter) {
		key = fmt.Sprintf("%s:", key)
	}
	return DatabaseKey(key)
}

func (ns Namespace) AddUserId(userId string) NamespaceUserId {
	key := fmt.Sprintf("%s:%s:{seq}", ns, userId)
	return NamespaceUserId(key)
}

func (nus NamespaceUserId) Build() DatabaseKey {
	key := string(nus)
	if !strings.Contains(key, SortableDelimiter) {
		key = fmt.Sprintf("%s:", key)
	}
	return DatabaseKey(key)
}

func (ns Namespace) AddNodeId(nodeId string) NamespaceNodeId {
	key := fmt.Sprintf("%s:%s:{seq}", ns, nodeId)
	return NamespaceNodeId(key)
}

func (nni NamespaceNodeId) Build() DatabaseKey {
	key := string(nni)
	if !strings.Contains(key, SortableDelimiter) {
		key = fmt.Sprintf("%s:", key)
	}
	return DatabaseKey(key)
}

// AddIPAddress adds an IP address segment to the key after validation
func (ns Namespace) AddHostAddress(host string) NamespaceNodeIdHost {
	key := fmt.Sprintf("%s:%s", ns, host)
	return NamespaceNodeIdHost(key)
}

func (nnh NamespaceNodeIdHost) Build() DatabaseKey {
	key := string(nnh)
	if !strings.Contains(key, SortableDelimiter) {
		key = fmt.Sprintf("%s:", key)
	}
	return DatabaseKey(key)
}

// AddTweetId adds a tweet ID segment to the key after validation
func (nsui NamespaceUserId) AddTweetId(tweetId string) NamespaceUserIdSeqTweetId {
	if tweetId == "" {
		return NamespaceUserIdSeqTweetId(nsui)
	}
	key := fmt.Sprintf("%s:%s", nsui, tweetId)
	return NamespaceUserIdSeqTweetId(key)
}

// AddFollowedId adds a followee ID segment to the key (reuses user ID validation)
func (pb *PrefixBuilder) AddWriterId(writerId string) *PrefixBuilder {
	// Skip processing if there's already an error
	//if pb.err != nil {
	//	return pb
	//}
	//
	//pb.key = fmt.Sprintf("%s:writer:%s", pb.key, writerId)
	return pb
}

func (pb *PrefixBuilder) AddReaderId(readerId string) *PrefixBuilder {
	// Skip processing if there's already an error
	//if pb.err != nil {
	//	return pb
	//}
	//
	//pb.key = fmt.Sprintf("%s:reader:%s", pb.key, readerId)
	return pb
}

// Build returns the final key string and any error that occurred during the chain
func (nust NamespaceUserIdSeqTweetId) Build() DatabaseKey {
	key := string(nust)
	if !strings.Contains(key, SortableDelimiter) {
		key = fmt.Sprintf("%s:", key)
	}
	return DatabaseKey(key)
}

func (k DatabaseKey) SortableValueKey(seqNum uint64) []byte {
	key := string(k)
	reverseSeq := fmt.Sprintf("%019d", math.MaxInt64-int64(seqNum))
	key = strings.ReplaceAll(key, "{seq}", reverseSeq)
	return []byte(key)
}

func (k DatabaseKey) KeyIndex() []byte {
	key := strings.ReplaceAll(string(k), ":", "_")
	key = strings.ReplaceAll(key, "{seq}", "none")
	return []byte(key)
}

func (k DatabaseKey) String() string {
	return string(k)
}
