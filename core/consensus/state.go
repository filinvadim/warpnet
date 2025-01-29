package consensus

import (
	"sync"
)

type SecretStorer interface{}

type store struct {
	mx        *sync.RWMutex
	state     State
	consensus Consensus
}

func NewSecretStore(consensus Consensus) SecretStorer {
	return &store{
		mx:        new(sync.RWMutex),
		state:     make(map[string]string),
		consensus: consensus,
	}
}

func (s *store) Commit(key string, secret string) (err error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.state.(map[string]string)[key] = secret

	s.state, err = s.consensus.CommitState(s.state)
	return err
}
