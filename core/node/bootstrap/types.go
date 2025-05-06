package bootstrap

import (
	"github.com/filinvadim/warpnet/core/consensus"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/event"
	"io"
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
	Run(m pubsub.PubsubServerNodeConnector, clientNode pubsub.PubsubClientNodeStreamer)
	PublishOwnerUpdate(ownerId string, msg event.Message) (err error)
	Close() error
}

type ConsensusProvider interface {
	Sync(node consensus.NodeServicesProvider) (err error)
	LeaderID() warpnet.WarpPeerID
	CommitState(newState consensus.KVState) (_ *consensus.KVState, err error)
	Shutdown()
}

type DistributedHashTableCloser interface {
	Close()
}

type ProviderCacheCloser interface {
	io.Closer
}

type ClientNodeStreamer interface {
	ClientStream(nodeId string, path string, data any) (_ []byte, err error)
}
