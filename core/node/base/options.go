package base

import (
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/libp2p/go-libp2p"
	libp2pConfig "github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"slices"
	"time"
)

func EnableAutoRelayWithStaticRelays(static []warpnet.PeerAddrInfo, currentNodeID warpnet.WarpPeerID) func() libp2p.Option {
	for i, info := range static {
		if info.ID == currentNodeID {
			static = slices.Delete(static, i, i+1)
			break
		}
	}
	return func() libp2p.Option {
		return func(cfg *libp2pConfig.Config) error {
			if len(static) == 0 {
				return nil
			}
			opts := []autorelay.Option{
				autorelay.WithBackoff(time.Minute * 5),
				autorelay.WithMaxCandidateAge(time.Minute * 5),
			}

			cfg.EnableAutoRelay = true
			cfg.AutoRelayOpts = append([]autorelay.Option{autorelay.WithStaticRelays(static)}, opts...)
			return nil
		}
	}
}

func DisableOption() func() libp2p.Option {
	return func() libp2p.Option {
		return func(cfg *libp2pConfig.Config) error {
			return nil
		}
	}
}
