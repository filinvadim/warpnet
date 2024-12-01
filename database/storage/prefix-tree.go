package storage

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	FixedKey      = "fixed"
	FixedRangeKey = FixedKey
)

type (
	Namespace   string
	ParentLayer string
	RangeLayer  string
	IdLayer     string

	DatabaseKey string
)

// PrefixBuilder is a struct that holds a key and any potential error
type PrefixBuilder struct {
	namespace Namespace
}

// NewPrefixBuilder creates a new PrefixBuilder instance
func NewPrefixBuilder(mandatoryNamespace string) Namespace {
	if mandatoryNamespace == "" {
		panic("namespace must not be empty")
	}
	return Namespace(mandatoryNamespace)
}

func (ns Namespace) Build() DatabaseKey {
	return build(string(ns))
}

func (ns Namespace) AddParent(mandatoryPrefix string) ParentLayer {
	if mandatoryPrefix == "" {
		panic("parent prefix must not be empty")
	}
	key := fmt.Sprintf("%s:%s", ns, mandatoryPrefix)
	return ParentLayer(key)
}

func (l ParentLayer) Build() DatabaseKey {
	return build(string(l))
}

type RangePrefix string

func (l ParentLayer) AddRange(mandatoryPrefix RangePrefix) RangeLayer {
	if mandatoryPrefix == "" {
		panic("range prefix must not be empty")
	}
	if mandatoryPrefix != FixedRangeKey {
		_, err := strconv.ParseInt(string(mandatoryPrefix), 10, 64)
		if err != nil {
			panic(fmt.Sprintf("invalid range prefix: %s", mandatoryPrefix))
		}
	}

	key := fmt.Sprintf("%s:%s", l, mandatoryPrefix)
	return RangeLayer(key)
}

func (l ParentLayer) AddReversedTimestamp(tm time.Time) RangeLayer {
	key := string(l)
	key = fmt.Sprintf("%s:%019d", key, math.MaxInt64-tm.Unix())
	return RangeLayer(key)
}
func (l RangeLayer) Build() DatabaseKey {
	return build(string(l))
}

func (l RangeLayer) AddId(mandatoryPrefix string) IdLayer {
	if mandatoryPrefix == "" {
		panic("id prefix must not be empty")
	}
	key := fmt.Sprintf("%s:%s", l, mandatoryPrefix)
	return IdLayer(key)
}

func (l IdLayer) Build() DatabaseKey {
	return build(string(l))
}

func build(s string) DatabaseKey {
	return DatabaseKey(s)
}

func (k DatabaseKey) IsEmpty() bool {
	return string(k) == ""
}
func (k DatabaseKey) String() string {
	return string(k)
}
func (k DatabaseKey) Bytes() []byte {
	return []byte(k)
}
func (k DatabaseKey) DropId() string {
	key := string(k)
	lastColon := strings.LastIndex(key, ":")
	if lastColon == -1 {
		// Если двоеточия нет, возвращаем оригинальную строку
		return key
	}
	// Возвращаем строку до последнего двоеточия
	return key[:lastColon]
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
