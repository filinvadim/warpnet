package consensus

import (
	"errors"
	"github.com/hashicorp/raft"
	log "github.com/sirupsen/logrus"
	"sync"
)

var errVoterNotFound = errors.New("voter not found")

type votersCache struct {
	mutex *sync.RWMutex
	m     map[raft.ServerID]raft.Server
}

func newVotersCache() *votersCache {
	pc := &votersCache{
		mutex: new(sync.RWMutex),
		m:     make(map[raft.ServerID]raft.Server),
	}

	return pc
}

func (d *votersCache) addVoter(key raft.ServerID, srv raft.Server) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.m[key] = srv
}

func (d *votersCache) removeVoter(key raft.ServerID) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.m, key)
}

func (d *votersCache) getVoter(key raft.ServerID) (_ raft.Server, err error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	voter, ok := d.m[key]
	if !ok {
		return raft.Server{}, errVoterNotFound
	}

	return voter, nil
}

func (d *votersCache) close() {
	d.m = nil
	log.Infoln("voters cache closed")
	return
}
