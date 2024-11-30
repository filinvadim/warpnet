package storage

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const NoneKey = "none"

type DatabaseKey string

type (
	Namespace  string
	KindLayer  string
	RangeLayer string
	IdLayer    string
)

// PrefixBuilder is a struct that holds a key and any potential error
type PrefixBuilder struct {
	namespace Namespace
}

// NewPrefixBuilder creates a new PrefixBuilder instance
func NewPrefixBuilder(ns string) Namespace {
	return Namespace(ns)
}

func (l Namespace) Build() DatabaseKey {
	return DatabaseKey(l)
}

func (ns Namespace) AddKind(prefix string) KindLayer {
	key := fmt.Sprintf("%s:%s", ns, prefix)
	return KindLayer(key)
}

func (l KindLayer) Build() DatabaseKey {
	return DatabaseKey(l)
}

func (l KindLayer) AddRange(prefix string) RangeLayer {
	key := fmt.Sprintf("%s:%s", l, prefix)
	return RangeLayer(key)
}

func (l KindLayer) AddReversedTimestamp(tm time.Time) RangeLayer {
	key := string(l)
	key = fmt.Sprintf("%s:%019d", key, math.MaxInt64-tm.Unix())
	return RangeLayer(key)
}
func (l RangeLayer) Build() DatabaseKey {
	return DatabaseKey(l)
}

func (l RangeLayer) AddId(prefix string) IdLayer {
	key := fmt.Sprintf("%s:%s", l, prefix)
	return IdLayer(key)
}

func (l IdLayer) Build() DatabaseKey {
	return DatabaseKey(l)
}

func (k DatabaseKey) String() string {
	return string(k)
}
func (k DatabaseKey) Bytes() []byte {
	return []byte(k)
}
func (k DatabaseKey) Cursor() string {
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
