package database

import (
	"encoding/binary"
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"os"
)

const ConsensusConfigNamespace = "/CONFIGS/"

var ErrConsensusKeyNotFound = errors.New("consensus key not found")

type ConsensusStorer interface {
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
	Sync() error
	Path() string
	NewReadTxn() (storage.WarpTxReader, error)
	Delete(key storage.DatabaseKey) error
}

type ConsensusRepo struct {
	db ConsensusStorer
}

func NewConsensusRepo(db ConsensusStorer) *ConsensusRepo {
	return &ConsensusRepo{db: db}
}

func (cr *ConsensusRepo) Sync() error {
	return cr.db.Sync()
}

func (cr *ConsensusRepo) SnapshotsPath() (path string) {
	return cr.db.Path() + "/snapshots"
}

func (cr *ConsensusRepo) Reset() error {
	txn, err := cr.db.NewReadTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	err = txn.IterateKeys(ConsensusConfigNamespace, func(key string) error {
		return cr.db.Delete(storage.DatabaseKey(key))
	})
	if err != nil {
		return err
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	return os.RemoveAll(cr.SnapshotsPath()) // NOTE this is only snapshots dir
}

// Set is used to set a key/value set outside of the raft log.
func (cr *ConsensusRepo) Set(key []byte, val []byte) error {
	return cr.db.Set(storage.DatabaseKey(append([]byte(ConsensusConfigNamespace), key...)), val)
}

// Get is used to retrieve a value from the k/v store by key
func (cr *ConsensusRepo) Get(key []byte) ([]byte, error) {
	prefix := storage.DatabaseKey(append([]byte(ConsensusConfigNamespace), key...))
	val, err := cr.db.Get(prefix)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrConsensusKeyNotFound
	}
	return val, err
}

// SetUint64 is like Set, but handles uint64 values
func (cr *ConsensusRepo) SetUint64(key []byte, val uint64) error {
	fullKey := storage.DatabaseKey(append([]byte(ConsensusConfigNamespace), key...))
	return cr.db.Set(fullKey, uint64ToBytes(val))
}

func (cr *ConsensusRepo) GetUint64(key []byte) (uint64, error) {
	fullKey := storage.DatabaseKey(append([]byte(ConsensusConfigNamespace), key...))
	val, err := cr.db.Get(fullKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, nil // intentionally!
	}

	return bytesToUint64(val), err
}

func (cr *ConsensusRepo) Close() error {
	return nil
}

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		var padded [8]byte
		copy(padded[8-len(b):], b)
		return binary.BigEndian.Uint64(padded[:])
	}
	return binary.BigEndian.Uint64(b)
}

// Converts a uint64 to a byte slice
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}
