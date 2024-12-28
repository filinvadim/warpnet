package storage

import (
	"errors"
	"github.com/filinvadim/warpnet/core/encrypting"

	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

const discardRatio = 0.5

var (
	ErrNotRunning = errors.New("DB is not running")
)

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence

	isRunning *atomic.Bool
	stopChan  chan struct{}

	opts   badger.Options
	dbPath string
}

func New(
	path string,
	isInMemory bool,
	dataFolder string,
) *DB {
	dbPath := path + dataFolder
	opts := badger.
		DefaultOptions(dbPath).
		WithSyncWrites(false).
		WithIndexCacheSize(256 << 20).
		WithCompression(options.Snappy).
		WithNumCompactors(2).
		WithLogger(nil)

	if isInMemory {
		opts.WithInMemory(true)
	}

	storage := &DB{
		badger: nil, stopChan: make(chan struct{}), isRunning: new(atomic.Bool),
		sequence: nil, opts: opts, dbPath: dbPath,
	}

	return storage
}

func (db *DB) Run(username, password string) (err error) {
	if db.isRunning.Load() {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("database username or password is empty")
	}
	hashSum := encrypting.ConvertToSHA256([]byte(username + "@" + password))
	db.opts.WithEncryptionKey(hashSum)

	db.badger, err = badger.Open(db.opts)
	if err != nil {
		return err
	}
	db.isRunning.Store(true)
	db.sequence, err = db.badger.GetSequence([]byte("SEQUENCE"), 100)
	if err != nil {
		return err
	}

	go db.runEventualGC()

	if username == "bootstrap" {
		log.Printf("database is running in bootstrap mode")
	} else {
		log.Printf("database is running in regular mode")
	}
	return nil
}

func (db *DB) runEventualGC() {
	log.Println("database garbage collection started")
	db.badger.RunValueLogGC(discardRatio)
	for {
		select {
		case <-time.After(time.Hour * 24):
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

func (db *DB) Path() string {
	return db.dbPath
}

func (db *DB) IsClosed() bool {
	return !db.isRunning.Load()
}

type IterKeysFunc func(key string) error

func (db *DB) IterateKeys(prefix DatabaseKey, handler IterKeysFunc) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	if strings.Contains(prefix.String(), FixedKey) {
		return errors.New("cannot iterate thru fixed key")
	}
	return db.badger.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
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
	})
}

type ListItem struct {
	Key   string
	Value []byte
}

func (db *DB) List(prefix DatabaseKey, limit *uint64, cursor *string) ([]ListItem, string, error) {
	var startCursor DatabaseKey
	if cursor != nil && *cursor != "" {
		startCursor = DatabaseKey(*cursor)
	}

	if limit == nil {
		defaultLimit := uint64(20)
		limit = &defaultLimit
	}

	items := make([]ListItem, 0, *limit) //
	cur, err := db.iterateKeysValues(
		prefix, startCursor, limit,
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

func (db *DB) iterateKeysValues(
	prefix DatabaseKey,
	startCursor DatabaseKey,
	limit *uint64,
	handler iterKeysValuesFunc,
) (cursor string, err error) {
	if !db.isRunning.Load() {
		return "", ErrNotRunning
	}
	if strings.Contains(prefix.String(), FixedKey) {
		return "", errors.New("cannot iterate thru fixed keys")
	}

	var lastKey DatabaseKey
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.PrefetchSize = 20

	err = db.badger.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opts)
		defer it.Close()

		p := prefix.Bytes()
		if !startCursor.IsEmpty() {
			p = startCursor.Bytes()
		}
		iterNum := 0
		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			p = prefix.Bytes() // starting point found

			item := it.Item()
			key := string(item.Key())

			if strings.Contains(key, FixedKey) {
				return nil
			}

			if iterNum > int(*limit) {
				lastKey = DatabaseKey(key)
				return nil
			}
			iterNum++

			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if err := handler(key, val); err != nil {
				return err
			}
		}
		if iterNum < int(*limit) {
			lastKey = ""
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return lastKey.DropId(), nil
}

func (db *DB) InnerDB() *badger.DB {
	return db.badger
}

func (db *DB) Set(key DatabaseKey, value []byte) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}

	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key.Bytes(), value)
		return txn.SetEntry(e)
	})
}

func (db *DB) SetWithTTL(key DatabaseKey, value []byte, ttl time.Duration) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(key.Bytes(), value)
		e.WithTTL(ttl)
		return txn.SetEntry(e)
	})
}

func (db *DB) Sync() error {
	return db.badger.Sync()
}

func (db *DB) Get(key DatabaseKey) ([]byte, error) {
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

func (db *DB) WriteTxn(f func(tx *badger.Txn) error) error {
	return db.txn(true, f)
}

func (db *DB) ReadTxn(f func(tx *badger.Txn) error) error {
	return db.txn(false, f)
}

func (db *DB) txn(isWrite bool, f func(tx *badger.Txn) error) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}

	txn := db.badger.NewTransaction(isWrite)
	defer txn.Discard()

	if err := f(txn); err != nil {
		return err
	}

	return txn.Commit()
}

func (db *DB) Delete(key DatabaseKey) error {
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

func (db *DB) NextSequence() (uint64, error) {
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

func (db *DB) Close() {
	log.Println("closing database...")
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
	db.isRunning.Store(false)
	if err := db.badger.Close(); err != nil {
		log.Println("database close: ", err)
	}
}
