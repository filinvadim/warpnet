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
	"github.com/filinvadim/dWighter/json"
	"github.com/labstack/gommon/log"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
)

var (
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
	db.sequence, err = db.badger.GetSequence([]byte("SEQUENCE"), 100)
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
			err := handler(string(item.Key()))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

type RawItem = []byte

func (db *DB) List(prefix DatabaseKey, limit *uint64, cursor *string) ([][]byte, string, error) {
	var startCursor DatabaseKey
	if cursor != nil && *cursor != "" {
		startCursor = DatabaseKey(*cursor)
	}

	if limit == nil {
		defaultLimit := uint64(20)
		limit = &defaultLimit
	}

	items := make([]RawItem, 0, 20)
	cur, err := db.iterateKeysValues(
		prefix, startCursor, limit,
		func(key string, value []byte) error {
			items = append(items, value)
			return nil
		})
	return jsonifyList(items), cur, err
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

func jsonifyList(items [][]byte) [][]byte {
	if len(items) == 0 {
		return items
	}
	items[0] = append(items[0], 0)
	copy(items[0][1:], items[0][0:])
	items[0][0] = byte('[')
	items[len(items)-1] = append(items[len(items)-1], ']')
	if !json.JSON.Valid(bytes.Join(items, []byte(","))) {
		return [][]byte{[]byte("invalid JSON array")}
	}
	return items
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

func (db *DB) Txn(f func(tx *badger.Txn) error) error {
	txn := db.badger.NewTransaction(true)
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
