package base

import (
	golog "github.com/ipfs/go-log/v2"
	"github.com/sirupsen/logrus"
)

var subsystems = []string{
	"autonat",
	"autonatv2",
	"autorelay",
	"basichost",
	"blankhost",
	"canonical-log",
	"connmgr",
	"dht",
	"dht.pb",
	"dht/RtDiversityFilter",
	"dht/RtRefreshManager",
	"dht/netsize",
	"discovery-backoff",
	"diversityFilter",
	"eventbus",
	"eventlog",
	"internal/nat",
	"ipns",
	"mdns",
	"nat",
	"net/identify",
	"p2p-circuit",
	"p2p-config",
	"p2p-holepunch",
	"peerstore",
	"peerstore/ds",
	"ping",
	"providers",
	"pstoremanager",
	"pubsub",
	"quic-transport",
	"quic-utils",
	"rcmgr",
	"relay",
	"reuseport-transport",
	"routedhost",
	"swarm2",
	"table",
	"tcp-demultiplex",
	"tcp-tpt",
	"test-logger",
	"upgrader",
	"websocket-transport",
	"webtransport",
	"webrtc-transport",
	"webrtc-transport-pion",
	"webrtc-udpmux",
}

func init() {
	level := logrus.GetLevel().String()
	_ = golog.SetLogLevel("autonatv2", level)
	_ = golog.SetLogLevel("p2p-holepunch", level)
	_ = golog.SetLogLevel("relay", level)
	_ = golog.SetLogLevel("nat", level)
	_ = golog.SetLogLevel("p2p-circuit", level)
	_ = golog.SetLogLevel("basichost", level)
	_ = golog.SetLogLevel("swarm2", "DEBUG")
	_ = golog.SetLogLevel("autorelay", level)
}
