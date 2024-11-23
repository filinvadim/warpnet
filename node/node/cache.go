package node

import (
	"github.com/filinvadim/dWighter/database"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sync"
)

type host = string

type nodeCache struct {
	nodes map[host]domain_gen.Node
	mutex *sync.RWMutex

	ownNode  domain_gen.Node
	nodeRepo *database.NodeRepo
}

func newNodeCache(nodeRepo *database.NodeRepo) (*nodeCache, error) {
	dc := &nodeCache{
		nodes:    make(map[host]domain_gen.Node),
		mutex:    new(sync.RWMutex),
		nodeRepo: nodeRepo,
	}
	//nodes, err := nodeRepo.List()
	//if err != nil {
	//	return nil, err
	//}
	//for _, n := range nodes {
	//	dc.nodes[n.Host] = n
	//} //  failed to init node service: node cache: db is not running

	return dc, nil
}

// AddNode adds a new IP address to the service
func (ds *nodeCache) AddNode(n domain_gen.Node) {
	var ok bool
	ds.mutex.Lock()
	_, ok = ds.nodes[n.Host]
	ds.nodes[n.Host] = n
	ds.mutex.Unlock()
	if ok {
		return
	}
	ds.nodeRepo.Create(&n)
}

// GetNodes retrieves the list of all IP addresses
func (ds *nodeCache) GetNodes() []domain_gen.Node {
	nodes := make([]domain_gen.Node, 0, len(ds.nodes))
	ds.mutex.RLock()
	for _, n := range ds.nodes {
		nodes = append(nodes, n)
	}
	ds.mutex.RUnlock()
	return nodes
}

// RemoveNode removes an IP address from the service
func (ds *nodeCache) RemoveNode(n *domain_gen.Node) {
	if n == nil {
		return
	}
	ds.mutex.Lock()
	delete(ds.nodes, n.Host)
	ds.mutex.Unlock()
	ds.nodeRepo.DeleteByHost(n.Host)
	ds.nodeRepo.DeleteByUserId(n.OwnerId)
}
