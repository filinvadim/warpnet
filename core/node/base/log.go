package base

import (
	golog "github.com/ipfs/go-log/v2"
	"github.com/sirupsen/logrus"
	"time"
)

func init() {
	level := logrus.GetLevel().String()
	_ = golog.SetLogLevel("autonatv2", level)
	_ = golog.SetLogLevel("p2p-holepunch", level)
	_ = golog.SetLogLevel("relay", level)
	_ = golog.SetLogLevel("nat", level)
	_ = golog.SetLogLevel("p2p-circuit", level)
	go func() {
		// too many initial failure logs during node setup
		time.Sleep(time.Minute * 2)
		_ = golog.SetLogLevel("autorelay", level)
	}()
}
