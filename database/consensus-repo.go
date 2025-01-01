package database

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/hashicorp/raft"
)

const (
	ConsensusLogsNamespace   = "LOGS:"
	ConsensusConfigNamespace = "CONFIGS:"
)

var (
	// ErrKeyNotFound is an error indicating a given key does not exist
	ErrKeyNotFound = errors.New("consensus key not found")
)

type ConsensusStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
	Sync() error
	Path() string
	IterateKeys(prefix storage.DatabaseKey, handler storage.IterKeysFunc) error
	InnerDB() *storage.WarpDB
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

func (cr *ConsensusRepo) SnapshotPath() string {
	return cr.db.Path() + "/snapshot"
}

// FirstIndex returns the first known index from the Raft log.
func (cr *ConsensusRepo) FirstIndex() (uint64, error) {
	var value uint64
	err := cr.db.IterateKeys(ConsensusLogsNamespace, func(key string) error {
		value = bytesToUint64([]byte(key)[1:])
		return nil
	})
	return value, err
}

// LastIndex returns the last known index from the Raft log.
func (cr *ConsensusRepo) LastIndex() (uint64, error) { // TODO
	var value uint64
	err := cr.db.InnerDB().View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Reverse:        true,
		})
		defer it.Close()

		it.Seek(append([]byte(ConsensusLogsNamespace), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff))
		if it.ValidForPrefix([]byte(ConsensusLogsNamespace)) { // TODO
			value = bytesToUint64(it.Item().Key()[1:])
		}
		return nil
	})
	return value, err
}

// GetLog gets a log entry from Badger at a given index.
func (cr *ConsensusRepo) GetLog(index uint64, log *raft.Log) error {
	prefix := append([]byte(ConsensusLogsNamespace), uint64ToBytes(index)...)
	val, err := cr.db.Get(storage.DatabaseKey(prefix))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrKeyNotFound
	}
	if err != nil {
		return err
	}
	return decode(val, log)
}

// StoreLog stores a single raft log.
func (cr *ConsensusRepo) StoreLog(log *raft.Log) error {
	val, err := encode(log)
	if err != nil {
		return err
	}
	prefix := append([]byte(ConsensusLogsNamespace), uint64ToBytes(log.Index)...)
	return cr.db.Set(storage.DatabaseKey(prefix), val.Bytes())
}

// StoreLogs stores a set of raft logs.
func (cr *ConsensusRepo) StoreLogs(logs []*raft.Log) error { // TODO
	last := 0
	errTooBig := error(nil)
	err := cr.db.WriteTxn(func(txn *badger.Txn) error {
		for i, log := range logs {
			var (
				key = append([]byte(ConsensusLogsNamespace), uint64ToBytes(log.Index)...)
			)
			val, err := encode(log)
			if err != nil {
				return err
			}
			err = txn.Set(key, val.Bytes())
			if errors.Is(err, badger.ErrTxnTooBig) {
				last = i
				errTooBig = err
				return nil
			}
			return err
		}
		return nil
	})
	if errTooBig != nil {
		errTooBig = nil
		return cr.StoreLogs(logs[last:])
	}
	return err
}

// DeleteRange deletes logs within a given range inclusively.
func (cr *ConsensusRepo) DeleteRange(min, max uint64) error {
	var (
		start     = append([]byte(ConsensusLogsNamespace), uint64ToBytes(min)...)
		end       = start
		errTooBig error
	)

	err := cr.db.WriteTxn(func(txn *badger.Txn) error {
		err := cr.db.IterateKeys(storage.DatabaseKey(start), func(key string) error {
			if bytesToUint64([]byte(key)[1:]) > max {
				return nil
			}
			err := txn.Delete([]byte(key))
			if errors.Is(err, badger.ErrTxnTooBig) {
				end = []byte(key)
				errTooBig = err
				return nil
			}
			return err
		})
		return err
	})
	if errTooBig != nil {
		errTooBig = nil
		return cr.DeleteRange(bytesToUint64(end[1:]), max)
	}
	return err
}

// Set is used to set a key/value set outside of the raft log.
func (cr *ConsensusRepo) Set(key []byte, val []byte) error {
	return cr.db.Set(storage.DatabaseKey(append([]byte(ConsensusConfigNamespace), key...)), val)
}

// Get is used to retrieve a value from the k/v store by key
func (cr *ConsensusRepo) Get(key []byte) ([]byte, error) {
	prefix := append([]byte(ConsensusConfigNamespace), key...)
	val, err := cr.db.Get(storage.DatabaseKey(prefix))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrKeyNotFound
	}
	return val, err
}

// ======================= INCREMENT =========================

// SetUint64 is like Set, but handles uint64 values
func (cr *ConsensusRepo) SetUint64(key []byte, val uint64) error {
	return cr.db.Set(storage.DatabaseKey(key), uint64ToBytes(val))
}

// GetUint64 is like Get, but handles uint64 values
func (cr *ConsensusRepo) GetUint64(key []byte) (uint64, error) {
	val, err := cr.db.Get(storage.DatabaseKey(key))
	return bytesToUint64(val), err
}
func (cr *ConsensusRepo) Close() error {
	return nil
}

// ======================= UTILS =========================

// Decode reverses the encode operation on a byte slice input
func decode(buf []byte, out interface{}) error {
	r := bytes.NewBuffer(buf)
	return json.JSON.NewDecoder(r).Decode(out)
}

// Encode writes an encoded object to a new bytes buffer
func encode(in interface{}) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	err := json.JSON.NewEncoder(buf).Encode(in)
	return buf, err
}

// Converts bytes to an integer
func bytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// Converts a uint to a byte slice
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}
