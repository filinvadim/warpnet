package database

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/hashicorp/raft"
	"io"
	"os"
)

const (
	ConsensusLogsNamespace   = "/LOGS/"
	ConsensusConfigNamespace = "/CONFIGS/"
)

var (
	// ErrKeyNotFound is an error indicating a given key does not exist
	ErrConsensusKeyNotFound = errors.New("consensus key not found")
	ErrStopIteration        = errors.New("stop iteration")
)

type ConsensusStorer interface {
	NewWriteTxn() (*storage.WarpWriteTxn, error)
	NewReadTxn() (*storage.WarpReadTxn, error)
	Set(key storage.DatabaseKey, value []byte) error
	Get(key storage.DatabaseKey) ([]byte, error)
	Sync() error
	Path() string
	InnerDB() *storage.WarpDB
}

type ConsensusRepo struct {
	db        ConsensusStorer
	fileStore *os.File
}

func NewConsensusRepo(db ConsensusStorer) (*ConsensusRepo, error) {
	fullPath := db.Path() + "/snapshot"
	f, err := os.OpenFile(fullPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		if f != nil {
			f.Close()
		}
		return nil, err
	}
	repo := &ConsensusRepo{db: db, fileStore: f}
	return repo, nil
}

func (cr *ConsensusRepo) Sync() error {
	return cr.db.Sync()
}

func (cr *ConsensusRepo) SnapshotFilestore() (file io.Writer, path string) {
	return cr.fileStore, cr.db.Path()
}

// FirstIndex returns the first known index from the Raft log.
func (cr *ConsensusRepo) FirstIndex() (uint64, error) {
	var value uint64

	txn, err := cr.db.NewReadTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	err = txn.IterateKeys(ConsensusLogsNamespace, func(key string) error {
		value = bytesToUint64([]byte(key[len(ConsensusLogsNamespace):]))
		return ErrStopIteration
	})
	if err != nil && !errors.Is(err, ErrStopIteration) {
		return 0, nil // intentionally
	}

	return value, nil
}

// LastIndex returns the last known index from the Raft log.
func (cr *ConsensusRepo) LastIndex() (uint64, error) {
	var value uint64
	txn, err := cr.db.NewReadTxn()
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	prefixMask := append([]byte(ConsensusLogsNamespace), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	err = txn.ReverseIterateKeys(storage.DatabaseKey(prefixMask), func(key string) error {
		value = bytesToUint64([]byte(key[len(ConsensusLogsNamespace):]))
		return ErrStopIteration
	})
	if err != nil && !errors.Is(err, ErrStopIteration) {
		return 0, err
	}
	if value == 0 {
		return 0, nil // intentionally!
	}
	return value, err
}

// GetLog gets a log entry from Badger at a given index.
func (cr *ConsensusRepo) GetLog(index uint64, log *raft.Log) error {
	prefix := append([]byte(ConsensusLogsNamespace), uint64ToBytes(index)...)
	val, err := cr.db.Get(storage.DatabaseKey(prefix))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrConsensusKeyNotFound
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
	txn, err := cr.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()
	for _, log := range logs {
		key := append([]byte(ConsensusLogsNamespace), uint64ToBytes(log.Index)...)
		val, err := encode(log)
		if err != nil {
			return err
		}
		if err = txn.Set(storage.DatabaseKey(key), val.Bytes()); errors.Is(err, badger.ErrTxnTooBig) {
			_ = txn.Commit()
			return err
		}
	}
	return txn.Commit()
}

// DeleteRange deletes logs within a given range inclusively.
func (cr *ConsensusRepo) DeleteRange(min, max uint64) error {
	var (
		start     = append([]byte(ConsensusLogsNamespace), uint64ToBytes(min)...)
		end       = start
		errTooBig error
	)

	txn, err := cr.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	readTxn, err := cr.db.NewReadTxn()
	if err != nil {
		return err
	}
	defer readTxn.Rollback()

	var keysToDelete [][]byte
	err = readTxn.IterateKeys(storage.DatabaseKey(start), func(key string) error {
		index := bytesToUint64([]byte(key)[len(ConsensusLogsNamespace):])
		if index > max {
			return nil
		}
		keysToDelete = append(keysToDelete, []byte(key))
		return nil
	})
	if err != nil {
		return err
	}

	if err := readTxn.Commit(); err != nil {
		return err
	}

	for _, key := range keysToDelete {
		if err := txn.Delete(storage.DatabaseKey(key)); errors.Is(err, badger.ErrTxnTooBig) {
			_ = txn.Commit()
			end = key
			errTooBig = err
			break
		}
	}

	// Если транзакция была слишком большой, продолжаем с последнего удаленного ключа
	if errTooBig != nil {
		nextMin := bytesToUint64(end[len(ConsensusLogsNamespace):])
		if nextMin > min { // Гарантия отсутствия бесконечной рекурсии
			return cr.DeleteRange(nextMin, max)
		}
	}
	return txn.Commit()
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
		return nil, ErrConsensusKeyNotFound
	}
	return val, err
}

// ======================= INCREMENT =========================

// SetUint64 is like Set, but handles uint64 values
func (cr *ConsensusRepo) SetUint64(key []byte, val uint64) error {
	fullKey := append([]byte(ConsensusConfigNamespace), key...)
	return cr.db.Set(storage.DatabaseKey(fullKey), uint64ToBytes(val))
}

func (cr *ConsensusRepo) GetUint64(key []byte) (uint64, error) {
	fullKey := append([]byte(ConsensusConfigNamespace), key...)
	val, err := cr.db.Get(storage.DatabaseKey(fullKey))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, nil // intentionally!
	}

	return bytesToUint64(val), err
}

func (cr *ConsensusRepo) Close() error {
	return cr.fileStore.Close()
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

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		var padded [8]byte
		copy(padded[8-len(b):], b) // Заполняем недостающие байты нулями
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
