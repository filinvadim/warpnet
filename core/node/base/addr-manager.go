package base

import (
	"github.com/filinvadim/warpnet/core/warpnet"
	"sync"
	"time"
)

type expiringAddress struct {
	expiration time.Time
	addr       warpnet.WarpAddress
}

// AddrManager manages additional addresses to be published via host.Addrs()
type AddrManager struct {
	mu         sync.Mutex
	extraAddrs map[string]expiringAddress
}

// New creates a new AddrManager
func NewAddressManager() *AddrManager {
	return &AddrManager{
		extraAddrs: make(map[string]expiringAddress),
	}
}

// Add adds a new multiaddr to be published (no duplicates)
func (m *AddrManager) Add(addr warpnet.WarpAddress, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extraAddrs[addr.String()] = expiringAddress{
		expiration: time.Now().Add(ttl),
		addr:       addr,
	}
}

// Factory returns a libp2p-compatible AddrsFactory function
func (m *AddrManager) Factory() func([]warpnet.WarpAddress) []warpnet.WarpAddress {
	return func(base []warpnet.WarpAddress) []warpnet.WarpAddress {
		m.mu.Lock()
		defer m.mu.Unlock()

		now := time.Now()

		out := make([]warpnet.WarpAddress, 0, len(base)+len(m.extraAddrs))
		out = append(out, base...)

		// avoid duplication with base
		existing := make(map[string]struct{}, len(base))
		for _, a := range base {
			existing[a.String()] = struct{}{}
		}
		for s, expAddr := range m.extraAddrs {
			if _, ok := existing[s]; !ok {
				if now.After(expAddr.expiration) {
					delete(m.extraAddrs, s)
					continue
				}
				out = append(out, expAddr.addr)
			}
		}

		existing = nil

		return out
	}
}
