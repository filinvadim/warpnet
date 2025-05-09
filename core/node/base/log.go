package base

import golog "github.com/ipfs/go-log/v2"

func init() {
	_ = golog.SetLogLevel("autorelay", "DEBUG")
	_ = golog.SetLogLevel("autonatv2", "DEBUG")
	_ = golog.SetLogLevel("p2p-holepunch", "DEBUG")
	_ = golog.SetLogLevel("relay", "DEBUG")
	_ = golog.SetLogLevel("nat", "DEBUG")
	_ = golog.SetLogLevel("p2p-circuit", "DEBUG")
}
