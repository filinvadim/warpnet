package discovery

import (
	"github.com/filinvadim/dWighter/database"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sync"
)

type host = string

// DiscoveryCache manages IP addresses
type discoveryCache struct {
	nodes map[host]domain_gen.Node
	mutex *sync.RWMutex

	ownNode domain_gen.Node
}

// NewDiscoveryService creates a new DiscoveryService instance
func newDiscoveryCache(nodeRepo *database.NodeRepo) (*discoveryCache, error) {
	dc := &discoveryCache{
		nodes: make(map[host]domain_gen.Node),
		mutex: new(sync.RWMutex),
	}
	own := nodeRepo.OwnNode()
	dc.nodes[PresetNodeAddress] = *own
	nodes, err := nodeRepo.List()
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		dc.nodes[n.Host] = n
	}

	return dc, nil
}

// AddNode adds a new IP address to the service
func (ds *discoveryCache) AddNode(n domain_gen.Node) {
	ds.mutex.Lock()
	ds.nodes[n.Host] = n
	ds.mutex.Unlock()
}

// GetNodes retrieves the list of all IP addresses
func (ds *discoveryCache) GetNodes() []domain_gen.Node {
	nodes := make([]domain_gen.Node, 0, len(ds.nodes))
	ds.mutex.RLock()
	for _, n := range ds.nodes {
		nodes = append(nodes, n)
	}
	ds.mutex.RUnlock()
	return nodes
}

// RemoveNode removes an IP address from the service
func (ds *discoveryCache) RemoveNode(n *domain_gen.Node) {
	if n == nil {
		return
	}
	ds.mutex.Lock()
	delete(ds.nodes, n.Host)
	ds.mutex.Unlock()
}
