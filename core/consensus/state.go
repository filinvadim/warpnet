package consensus

import (
	"sync"
)

type stateStore struct {
	mx        *sync.RWMutex
	state     State
	consensus *Consensus
}

func NewStateStore(stateBase map[string]string, consensus *Consensus) *stateStore {
	return &stateStore{
		mx:        new(sync.RWMutex),
		state:     stateBase,
		consensus: consensus,
	}
}

func (s *stateStore) Commit(key string, secret string) (err error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.state.(map[string]string)[key] = secret

	s.state, err = s.consensus.CommitState(s.state)
	return err
}
