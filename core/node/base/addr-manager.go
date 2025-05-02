package base

import (
	"github.com/filinvadim/warpnet/core/warpnet"
	"sync"
	"time"
)

// AddrManager manages additional addresses to be published via host.Addrs()
type AddrManager struct {
	mu         sync.Mutex
	extraAddrs map[string]time.Time
}

// NewAddressManager creates a new AddrManager
func NewAddressManager() *AddrManager {
	return &AddrManager{
		extraAddrs: make(map[string]time.Time),
	}
}

// Add adds a new multiaddr to be published (no duplicates)
func (m *AddrManager) Add(addr warpnet.WarpAddress, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extraAddrs[addr.String()] = time.Now().Add(ttl)
}

// Factory returns a libp2p-compatible AddrsFactory function
func (m *AddrManager) Factory() func([]warpnet.WarpAddress) []warpnet.WarpAddress {
	return func(base []warpnet.WarpAddress) (out []warpnet.WarpAddress) {
		m.mu.Lock()
		defer m.mu.Unlock()

		if len(m.extraAddrs) == 0 {
			return base
		}

		out = make([]warpnet.WarpAddress, 0, len(base)+len(m.extraAddrs))
		out = append(out, base...)

		now := time.Now()
		for addr, tm := range m.extraAddrs {
			if now.After(tm) {
				delete(m.extraAddrs, addr)
				continue
			}

			mAddr, _ := warpnet.NewMultiaddr(addr)
			out = append(out, mAddr)
		}
		return out
	}
}
