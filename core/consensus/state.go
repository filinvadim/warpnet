package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/json"
	"github.com/hashicorp/raft"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
	"io"
	"sync"
)

type KVState map[string]string

type FSM struct {
	state     *KVState
	prevState KVState

	initialized bool

	mux *sync.Mutex

	validators []ConsensusValidatorFunc
}

type ConsensusValidatorFunc func(map[string]string) error

func newFSM(validators ...ConsensusValidatorFunc) *FSM {
	state := KVState{"genesis": ""}
	return &FSM{
		state:       &state,
		prevState:   KVState{},
		initialized: false,
		mux:         new(sync.Mutex),
		validators:  validators,
	}
}

// Apply is invoked by Raft once a log entry is commited. Do not use directly.
func (fsm *FSM) Apply(rlog *raft.Log) (result interface{}) {
	fsm.mux.Lock()
	defer fsm.mux.Unlock()
	defer func() {
		if r := recover(); r != nil {
			*fsm.state = fsm.prevState
			result = errors.New("fsm apply panic: rollback")
		}
	}()

	var newState = make(map[string]string, 1)
	if err := msgpack.Unmarshal(rlog.Data, &newState); err != nil {
		return fmt.Errorf("failed to decode log: %w", err)
	}

	for _, v := range fsm.validators {
		if err := v(newState); err != nil {
			log.Errorf("failed to apply validator: %v", err)
			return err
		}
	}

	fsm.prevState = make(map[string]string, len(*fsm.state))
	for k, v := range *fsm.state {
		fsm.prevState[k] = v
	}

	for k, v := range newState {
		(*fsm.state)[k] = v
	}

	fsm.initialized = true
	return fsm.state
}

// Snapshot encodes the current state so that we can save a snapshot.
func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	fsm.mux.Lock()
	defer fsm.mux.Unlock()
	if !fsm.initialized {
		log.Error("fsm: snapshot uninitialized state")
		return nil, errors.New("fsm: snapshot uninitialized state")
	}

	buf := new(bytes.Buffer)
	err := json.JSON.NewEncoder(buf).Encode(fsm.state)
	if err != nil {
		return nil, err
	}

	return &fsmSnapshot{state: buf}, nil
}

// Restore takes a snapshot and sets the current state from it.
func (fsm *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()
	fsm.mux.Lock()
	defer fsm.mux.Unlock()

	err := json.JSON.NewDecoder(reader).Decode(fsm.state)
	if err != nil {
		log.Errorf("fsm: decoding snapshot: %s", err)
		return err
	}

	fsm.prevState = make(map[string]string, len(*fsm.state))
	fsm.initialized = true

	return nil
}

type fsmSnapshot struct {
	state *bytes.Buffer
}

// Persist writes the snapshot (a serialized state) to a raft.SnapshotSink.
func (snap *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	_, err := io.Copy(sink, snap.state)
	if err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (snap *fsmSnapshot) Release() {
	log.Debugln("fsm: releasing snapshot")
}
