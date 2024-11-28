package storage

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/filinvadim/dWighter/config"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/labstack/gommon/log"
	"math/rand/v2"
	"sync/atomic"
	"time"
)

var (
	ErrStopIteration = errors.New("stop iteration")
	ErrWrongPassword = errors.New("wrong password")
	ErrNotRunning    = errors.New("DB is not running")
)

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence

	isRunning *atomic.Bool
	stopChan  chan struct{}

	opts badger.Options
}

func New(
	path string,
	isInMemory bool,
) *DB {
	opts := badger.
		DefaultOptions(path + config.DatabaseFolder).
		WithSyncWrites(false).
		WithIndexCacheSize(256 << 20).
		WithCompression(options.Snappy).
		WithNumCompactors(2).
		WithLogger(nil)

	if isInMemory {
		opts.WithInMemory(isInMemory)
	}

	storage := &DB{
		badger: nil, stopChan: make(chan struct{}), isRunning: new(atomic.Bool),
		sequence: nil, opts: opts,
	}

	return storage
}

func (db *DB) Run(username, password string) (token string, err error) {
	if db.isRunning.Load() {
		return "", nil
	}
	hashSum := crypto.ConvertToSHA256([]byte(username + "@" + password))
	db.opts.WithEncryptionKey(hashSum)

	db.badger, err = badger.Open(db.opts)
	if err != nil {
		return "", err
	}

	db.isRunning.Store(true)
	fmt.Println("DATABASE IS RUNNING!")
	db.sequence, err = db.badger.GetSequence([]byte("SEQUENCE:unified"), 100)
	if err != nil {
		return "", err
	}

	randChar := string(uint8(rand.Uint()))

	feed := []byte(username + "@" + password + "@" + randChar + "@" + time.Now().String())
	sessionToken := base64.StdEncoding.EncodeToString(crypto.ConvertToSHA256(feed))
	go db.runEventualGC()
	return sessionToken, nil
}

func (db *DB) runEventualGC() {
	fmt.Println("badger GC started")
	db.badger.RunValueLogGC(0.5)
	for {
		select {
		case <-time.After(time.Hour * 24):
			for {
				err := db.badger.RunValueLogGC(0.5)
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

type IterKeysFunc func(key string) error

func (db *DB) IterateKeys(prefix string, handler IterKeysFunc) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		p := []byte(prefix)
		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()
			err := handler(string(item.Key()))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

type RawItem = []byte

func (db *DB) List(prefix string, limit *uint64, cursor *string) ([]byte, string, error) {
	if !db.isRunning.Load() {
		return nil, "", ErrNotRunning
	}
	if limit == nil {
		defaultLimit := uint64(20)
		limit = &defaultLimit
	}

	var (
		lastCursor  string
		startCursor = prefix
	)
	if cursor != nil && *cursor != "" {
		startCursor = *cursor
	}

	items := make([]RawItem, 0, *limit)
	err := db.iterateKeysValues(startCursor, func(key string, value []byte) error {
		if !IsValidForPrefix(key, prefix) {
			return nil
		}
		if len(items) >= int(*limit) {
			lastCursor = key
			return ErrStopIteration
		}
		items = append(items, value)
		return nil
	})
	if err != nil && !errors.Is(err, ErrStopIteration) {
		return nil, "", err
	}
	if len(items) < int(*limit) {
		lastCursor = ""
	}
	return listify(items), lastCursor, nil
}

func listify(items [][]byte) []byte {
	itemsList := bytes.Join(items, []byte(`,`))
	itemsList = append(itemsList, 0)
	copy(itemsList[1:], itemsList[0:])
	itemsList[0] = byte('[')
	return append(itemsList, ']')
}

type iterKeysValuesFunc func(key string, val []byte) error

func (db *DB) iterateKeysValues(prefix string, handler iterKeysValuesFunc) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		p := []byte(prefix)
		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()
			key := string(item.Key())
			err := item.Value(func(val []byte) error {
				// Call the handler function to process the key-value pair
				return handler(key, val)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *DB) Set(key string, value []byte) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), value)
		return txn.SetEntry(e)
	})
}

func (db *DB) Get(key string) ([]byte, error) {
	if !db.isRunning.Load() {
		return nil, ErrNotRunning
	}
	var value []byte
	err := db.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (db *DB) Update(key string, newValue []byte) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), newValue)
		return txn.SetEntry(e)
	})
}

func (db *DB) Txn(f func(tx *badger.Txn) error) error {
	txn := db.badger.NewTransaction(true)
	defer txn.Discard()

	if err := f(txn); err != nil {
		return err
	}

	return txn.Commit()
}

func (db *DB) Delete(key string) error {
	if !db.isRunning.Load() {
		return ErrNotRunning
	}
	return db.badger.Update(func(txn *badger.Txn) error {
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
		err := db.badger.RunValueLogGC(0.5)
		if errors.Is(err, badger.ErrNoRewrite) {
			return
		}
	}
}

func (db *DB) Close() {
	close(db.stopChan)
	if db.sequence != nil {
		db.sequence.Release()
	}
	if db.badger == nil {
		return
	}
	if !db.isRunning.Load() {
		return
	}
	db.isRunning.Store(false)
	if err := db.badger.Close(); err != nil {
		log.Error(err)
	}
}
