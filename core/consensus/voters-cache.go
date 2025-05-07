package consensus

import (
	"errors"
	"fmt"
	"github.com/hashicorp/raft"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var errVoterNotFound = errors.New("consensus: voter not found")

type voterTimed struct {
	voter   raft.Server
	addedAt time.Time
}

type votersCache struct {
	mutex *sync.RWMutex
	m     map[raft.ServerID]voterTimed
}

func newVotersCache() *votersCache {
	pc := &votersCache{
		mutex: new(sync.RWMutex),
		m:     make(map[raft.ServerID]voterTimed),
	}

	return pc
}

func (d *votersCache) addVoter(key raft.ServerID, srv raft.Server) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.m[key] = voterTimed{srv, time.Now()}
}

var ErrTooSoonToRemoveVoter = errors.New("consensus:too soon to remove voter")

func (d *votersCache) removeVoter(key raft.ServerID) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	v, ok := d.m[key]
	if !ok {
		return nil
	}

	if time.Since(v.addedAt) < (time.Minute * 5) {
		return ErrTooSoonToRemoveVoter // flapping prevention
	}

	delete(d.m, key)
	return nil
}

func (d *votersCache) getVoter(key raft.ServerID) (_ raft.Server, err error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	v, ok := d.m[key]
	if !ok {
		return raft.Server{}, errVoterNotFound
	}

	return v.voter, nil
}

func (d *votersCache) print() {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	log.Info("consensus: voters list in cache:")
	for k := range d.m {
		fmt.Printf("========== %s", k)
	}
}

func (d *votersCache) close() {
	d.m = nil
	log.Infoln("consensus: voters cache closed")
	return
}
