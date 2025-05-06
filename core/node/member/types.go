package member

import (
	"github.com/filinvadim/warpnet/core/consensus"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"io"
	"time"
)

type DiscoveryHandler interface {
	HandlePeerFound(pi warpnet.PeerAddrInfo)
	Run(n discovery.DiscoveryInfoStorer) error
	Close()
}

type MDNSStarterCloser interface {
	Start(n mdns.NodeConnector)
	Close()
}

type PubSubProvider interface {
	SubscribeUserUpdate(userId string) (err error)
	UnsubscribeUserUpdate(userId string) (err error)
	Run(m pubsub.PubsubServerNodeConnector, clientNode pubsub.PubsubClientNodeStreamer)
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
	Close() error
}

type UserFetcher interface {
	Get(userId string) (user domain.User, err error)
	Update(userId string, newUser domain.User) (domain.User, error)
	GetByNodeID(nodeID string) (user domain.User, err error)
}

type ConsensusProvider interface {
	Sync(node consensus.NodeServicesProvider) (err error)
	LeaderID() warpnet.WarpPeerID
	CommitState(newState consensus.KVState) (_ *consensus.KVState, err error)
	Shutdown()
	AskUserValidation(user domain.User) error
	Stats() map[string]string
}

type DistributedHashTableCloser interface {
	Close()
}

type ProviderCacheCloser interface {
	io.Closer
}

type AuthProvider interface {
	GetOwner() domain.Owner
	SessionToken() string
}

type UserProvider interface {
	Create(user domain.User) (domain.User, error)
	ValidateUser(k, v string) error
	GetByNodeID(nodeID string) (user domain.User, err error)
	Get(userId string) (user domain.User, err error)
	List(limit *uint64, cursor *string) ([]domain.User, string, error)
	Update(userId string, newUser domain.User) (updatedUser domain.User, err error)
	GetBatch(userIds ...string) (users []domain.User, err error)
}

type ClientNodeStreamer interface {
	ClientStream(nodeId string, path string, data any) (_ []byte, err error)
}

type ConsensusStorer interface {
	Reset() error
}

type FollowStorer interface {
	GetFollowersCount(userId string) (uint64, error)
	GetFolloweesCount(userId string) (uint64, error)
	Follow(fromUserId, toUserId string, event domain.Following) error
	Unfollow(fromUserId, toUserId string) error
	GetFollowers(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
	GetFollowees(userId string, limit *uint64, cursor *string) ([]domain.Following, string, error)
}

type Storer interface {
	NewWriteTxn() (storage.WarpTxWriter, error)
	NewReadTxn() (storage.WarpTxReader, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	GetExpiration(key storage.DatabaseKey) (uint64, error)
	GetSize(key storage.DatabaseKey) (int64, error)
	Sync() error
	IsClosed() bool
	InnerDB() *storage.WarpDB
	SetWithTTL(key storage.DatabaseKey, value []byte, ttl time.Duration) error
	Set(key storage.DatabaseKey, value []byte) error
	Delete(key storage.DatabaseKey) error
	Path() string
	Stats() map[string]string
	IsFirstRun() bool
}
