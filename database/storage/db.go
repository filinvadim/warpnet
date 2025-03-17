package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/filinvadim/warpnet/security"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

/*
  BadgerDB is a high-performance, embedded key-value database
  written in Go, utilizing LSM-trees (Log-Structured Merge-Trees) for efficient
  data storage and processing. It is designed for high-load scenarios that require
  minimal latency and high throughput.

  Key Features:
    - Embedded: Operates within the application without the need for a separate database server.
    - Key-Value Store: Enables storing and retrieving data by key using efficient indexing.
    - LSM Architecture: Provides high write speed due to log-structured data storage.
    - Zero GC Overhead: Minimizes garbage collection impact by directly working with mmap and byte slices.
    - ACID Transactions: Supports transactions with snapshot isolation.
    - In-Memory and Disk Mode: Allows storing data in RAM or on SSD/HDD.
    - Low Resource Consumption: Suitable for embedded systems and server applications with limited memory.

  BadgerDB is used in cases where:
    - High Performance is required: It is faster than traditional disk-based databases (e.g., BoltDB) due to the LSM structure.
    - Embedded Storage is needed: No need to run a separate database server (unlike Redis, PostgreSQL, etc.).
    - Efficient Streaming Writes are required: Suitable for logs, caches, message brokers, and other write-intensive workloads.
    - Transaction Support is necessary: Allows safely executing multiple operations within a single transaction.
    - Large Data Volumes are handled: Supports sharding and disk offloading, useful for processing massive datasets.
    - Flexibility is key: Easily integrates into distributed systems and P2P applications.

  BadgerDB is especially useful for systems where high write speed, low overhead, and the ability to operate without an external database server are critical.
  https://github.com/dgraph-io/badger
*/

const discardRatio = 0.5

var (
	ErrNotRunning    = errors.New("DB is not running")
	ErrKeyNotFound   = badger.ErrKeyNotFound
	ErrWrongPassword = errors.New("wrong username or password")
)

type (
	WarpDB = badger.DB
)

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence

	isRunning *atomic.Bool
	stopChan  chan struct{}

	storedOpts badger.Options
	dbPath     string
	isDirEmpty bool
}

type WarpDBLogger interface {
	Errorf(string, ...interface{})
	Warningf(string, ...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
}

func New(
	path string,
	isInMemory bool,
	dataFolder string,
) (*DB, func(), error) {
	dbPath := filepath.Join(path, dataFolder)
	opts := badger.
		DefaultOptions(dbPath).
		WithSyncWrites(false).
		WithIndexCacheSize(256 << 20).
		WithCompression(options.Snappy).
		WithNumCompactors(2).
		WithLoggingLevel(badger.WARNING)
	if isInMemory {
		opts.WithInMemory(true)
	}

	isDirEmpty, err := isDirectoryEmpty(dbPath)
	if err != nil {
		return nil, nil, err
	}

	storage := &DB{
		badger: nil, stopChan: make(chan struct{}), isRunning: new(atomic.Bool),
		sequence: nil, storedOpts: opts, dbPath: dbPath, isDirEmpty: isDirEmpty,
	}

	return storage, storage.Close, nil
}

func isDirectoryEmpty(dirPath string) (bool, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			err := os.Mkdir(dirPath, 0750)
			return true, err
		}
		return false, fmt.Errorf("is dir empty: %w", err)
	}

	return len(dirEntries) == 0, nil
}

func (db *DB) IsFirstRun() bool {
	return db.isDirEmpty
}

func (db *DB) Run(username, password string) (err error) {
	if username == "" || password == "" {
		return errors.New("database username or password is empty")
	}
	hashSum := security.ConvertToSHA256([]byte(username + "@" + password))
	execOpts := db.storedOpts.WithEncryptionKey(hashSum)

	db.badger, err = badger.Open(execOpts)
	if errors.Is(err, badger.ErrEncryptionKeyMismatch) {
		return ErrWrongPassword
	}
	if err != nil {
		return err
	}
	db.isRunning.Store(true)
	db.sequence, err = db.badger.GetSequence([]byte("SEQUENCE"), 100)
	if err != nil {
		return err
	}

	go db.runEventualGC()

	return nil
}

func (db *DB) runEventualGC() {
	log.Infoln("database garbage collection started")
	_ = db.badger.RunValueLogGC(discardRatio)
	for {
		isEmpty, err := isDirectoryEmpty(db.dbPath)
		if isEmpty && err == nil {
			log.Fatalln("database folder was emptied")
		}
		select {
		case <-time.After(time.Hour):
			for {
				err := db.badger.RunValueLogGC(discardRatio)
				if errors.Is(err, badger.ErrNoRewrite) {
					break
				}
				time.Sleep(time.Second)
			}
		case <-db.stopChan:
			return
		}
	}
}

func (db *DB) Set(key DatabaseKey, value []byte) error {
	if db == nil {
		return ErrNotRunning
	}
	if !db.isRunning.Load() {
		return ErrNotRunning
	}

	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key.Bytes(), value)
		return txn.SetEntry(e)
	})
}

func (db *DB) SetWithTTL(key DatabaseKey, value []byte, ttl time.Duration) error {
	if db == nil {
		return ErrNotRunning
	}
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key.Bytes(), value)
		e.WithTTL(ttl)
		return txn.SetEntry(e)
	})
}

func (db *DB) Get(key DatabaseKey) ([]byte, error) {
	if db == nil {
		return nil, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return nil, ErrNotRunning
	}

	var result []byte
	err := db.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key.Bytes())
		if err != nil {
			return err
		}

		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		result = append([]byte{}, val...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (db *DB) GetExpiration(key DatabaseKey) (uint64, error) {
	if db == nil {
		return 0, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return 0, ErrNotRunning
	}

	var result uint64
	err := db.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key.Bytes())
		if err != nil {
			return err
		}

		result = item.ExpiresAt()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (db *DB) GetSize(key DatabaseKey) (int64, error) {
	if db == nil {
		return 0, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return 0, ErrNotRunning
	}

	var result int64
	err := db.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key.Bytes())
		if err != nil {
			return err
		}

		result = item.ValueSize()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (db *DB) Delete(key DatabaseKey) error {
	if db == nil {
		return ErrNotRunning
	}
	if !db.isRunning.Load() {
		return ErrNotRunning
	}

	return db.badger.Update(func(txn *badger.Txn) error {
		if err := txn.Delete(key.Bytes()); err != nil {
			return err
		}
		return txn.Delete([]byte(key))
	})
}

type WarpTxn = badger.Txn

type WarpWriteTxn struct {
	txn *badger.Txn
}

type WarpReadTxn struct {
	txn *badger.Txn
}

func (db *DB) NewWriteTxn() (*WarpWriteTxn, error) {
	if db == nil {
		return nil, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return nil, ErrNotRunning
	}
	return &WarpWriteTxn{db.badger.NewTransaction(true)}, nil
}

func (t *WarpWriteTxn) Set(key DatabaseKey, value []byte) error {
	err := t.txn.Set(key.Bytes(), value)
	if err != nil {
		return err
	}
	return nil
}

func (t *WarpWriteTxn) Get(key DatabaseKey) ([]byte, error) {
	var result []byte
	item, err := t.txn.Get(key.Bytes())
	if err != nil {
		return nil, err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	result = append([]byte{}, val...)

	return result, nil
}

func (t *WarpWriteTxn) SetWithTTL(key DatabaseKey, value []byte, ttl time.Duration) error {
	e := badger.NewEntry(key.Bytes(), value)
	e.WithTTL(ttl)

	err := t.txn.SetEntry(e)
	if err != nil {
		return err
	}
	return nil
}

func (t *WarpWriteTxn) BatchSet(data []ListItem) (err error) {
	var (
		lastIndex int
		isTooBig  bool
	)
	for i, item := range data {
		key := item.Key
		value := item.Value
		err := t.txn.Set([]byte(key), value)
		if errors.Is(err, badger.ErrTxnTooBig) {
			isTooBig = true
			lastIndex = i
			_ = t.txn.Commit() // force commit in the middle of iteration
			break
		}
		if err != nil {
			return err
		}
	}
	if isTooBig {
		leftovers := data[lastIndex:]
		data = nil
		err = t.BatchSet(leftovers)
	}
	return err
}

func (t *WarpWriteTxn) Delete(key DatabaseKey) error {
	if err := t.txn.Delete(key.Bytes()); err != nil {
		return err
	}
	return t.txn.Delete([]byte(key))
}

func (t *WarpWriteTxn) Increment(key DatabaseKey) (uint64, error) {
	return increment(t.txn, key.Bytes(), 1)
}

func (t *WarpWriteTxn) Decrement(key DatabaseKey) (uint64, error) {
	return increment(t.txn, key.Bytes(), -1)
}

func increment(txn *badger.Txn, key []byte, incVal int64) (uint64, error) {
	var newValue int64

	item, err := txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		newValue = 1
		return uint64(newValue), txn.Set(key, encodeInt64(newValue))
	}
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return uint64(newValue), err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return uint64(newValue), err
	}
	newValue = decodeInt64(val) + incVal
	if newValue < 0 {
		newValue = 0
	}

	err = txn.Set(key, encodeInt64(newValue))
	return uint64(newValue), err
}

func (t *WarpWriteTxn) Commit() error {
	return t.txn.Commit()
}

func (t *WarpWriteTxn) Rollback() {
	t.txn.Discard()
}

// =========== READ ===============================

func (db *DB) NewReadTxn() (*WarpReadTxn, error) {
	if db == nil {
		return nil, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return nil, ErrNotRunning
	}
	return &WarpReadTxn{db.badger.NewTransaction(false)}, nil
}

type IterKeysFunc func(key string) error

func (t *WarpReadTxn) IterateKeys(prefix DatabaseKey, handler IterKeysFunc) error {
	if strings.Contains(prefix.String(), FixedKey) {
		return errors.New("cannot iterate thru fixed key")
	}
	opts := badger.DefaultIteratorOptions
	it := t.txn.NewIterator(opts)
	defer it.Close()
	p := []byte(prefix)

	for it.Seek(p); it.ValidForPrefix(p); it.Next() {
		item := it.Item()
		err := handler(string(item.KeyCopy(nil)))
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *WarpReadTxn) ReverseIterateKeys(prefix DatabaseKey, handler IterKeysFunc) error {
	if strings.Contains(prefix.String(), FixedKey) {
		return errors.New("cannot iterate thru fixed key")
	}
	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	it := t.txn.NewIterator(opts)
	defer it.Close()
	p := []byte(prefix)

	fmt.Println("REVERSE", prefix)

	for it.Seek(p); it.ValidForPrefix(p); it.Next() {
		item := it.Item()
		fmt.Println("KEY", item.KeyCopy(nil))

		err := handler(string(item.KeyCopy(nil)))
		if err != nil {
			return err
		}
	}
	return nil
}

type ListItem struct {
	Key   string
	Value []byte
}

func (t *WarpReadTxn) List(prefix DatabaseKey, limit *uint64, cursor *string) ([]ListItem, string, error) {
	var startCursor DatabaseKey
	if cursor != nil && *cursor != "" {
		startCursor = DatabaseKey(*cursor)
	}

	if limit == nil {
		defaultLimit := uint64(20)
		limit = &defaultLimit
	}

	items := make([]ListItem, 0, *limit) //
	cur, err := iterateKeysValues(
		t.txn, prefix, startCursor, limit,
		func(key string, value []byte) error {
			items = append(items, ListItem{
				Key:   key,
				Value: value,
			})
			return nil
		})
	return items, cur, err
}

type iterKeysValuesFunc func(key string, val []byte) error

func iterateKeysValues(
	txn *badger.Txn,
	prefix DatabaseKey,
	startCursor DatabaseKey,
	limit *uint64,
	handler iterKeysValuesFunc,
) (cursor string, err error) {
	if strings.Contains(prefix.String(), FixedKey) {
		return "", errors.New("cannot iterate thru fixed keys")
	}

	var lastKey DatabaseKey
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.PrefetchSize = 20

	it := txn.NewIterator(opts)
	defer it.Close()

	p := prefix.Bytes()
	if !startCursor.IsEmpty() {
		p = startCursor.Bytes()
	}
	iterNum := uint64(0)
	for it.Seek(p); it.ValidForPrefix(p); it.Next() {
		p = prefix.Bytes() // starting point found

		item := it.Item()
		key := string(item.Key())

		if strings.Contains(key, FixedKey) {
			return "", nil
		}

		if iterNum > *limit {
			lastKey = DatabaseKey(key)
			return "", nil
		}
		iterNum++

		val, err := item.ValueCopy(nil)
		if err != nil {
			return "", err
		}
		if err := handler(key, val); err != nil {
			return "", err
		}
	}
	if iterNum < *limit {
		lastKey = ""
	}
	return lastKey.DropId(), nil
}

func (t *WarpReadTxn) Get(key DatabaseKey) ([]byte, error) {
	var result []byte
	item, err := t.txn.Get(key.Bytes())
	if err != nil {
		return nil, err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	result = append([]byte{}, val...)

	return result, nil
}

func (t *WarpReadTxn) BatchGet(keys ...DatabaseKey) ([]ListItem, error) {
	result := make([]ListItem, 0, len(keys))

	for _, key := range keys {
		item, err := t.txn.Get([]byte(key))
		if errors.Is(err, badger.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		it := ListItem{
			Key:   key.String(),
			Value: val,
		}
		result = append(result, it)
	}

	return result, nil
}

func (t *WarpReadTxn) Commit() error {
	return t.txn.Commit()
}

func (t *WarpReadTxn) Rollback() {
	t.txn.Discard()
}

// =====================================================

func (db *DB) NextSequence() (uint64, error) {
	if db == nil {
		return 0, ErrNotRunning
	}
	if !db.isRunning.Load() {
		return 0, ErrNotRunning
	}
	num, err := db.sequence.Next()
	if err != nil {
		return 0, err
	}
	if num != 0 {
		return num, nil
	}

	return db.sequence.Next()
}

func (db *DB) GC() {
	if db == nil {
		panic(ErrNotRunning)
	}
	if !db.isRunning.Load() {
		return
	}
	for {
		err := db.badger.RunValueLogGC(discardRatio)
		if errors.Is(err, badger.ErrNoRewrite) {
			return
		}
	}
}

func (db *DB) InnerDB() *badger.DB {
	if db == nil {
		panic(ErrNotRunning)
	}
	return db.badger
}

func (db *DB) Sync() error {
	if db == nil {
		return ErrNotRunning
	}
	return db.badger.Sync()
}

func (db *DB) Path() string {
	if db == nil {
		return ""
	}
	return db.dbPath
}

func (db *DB) IsClosed() bool {
	if db == nil {
		return true
	}
	return !db.isRunning.Load()
}

func encodeInt64(n int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}

func decodeInt64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}

func (db *DB) Close() {
	if db == nil {
		return
	}
	log.Infoln("closing database...")
	close(db.stopChan)
	if db.sequence != nil {
		_ = db.sequence.Release()
	}
	if db.badger == nil {
		return
	}
	if !db.isRunning.Load() {
		return
	}

	_ = db.Sync()
	if err := db.badger.Close(); err != nil {
		log.Infoln("database close: ", err)
		return
	}
	db.isRunning.Store(false)
}
