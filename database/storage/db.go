package storage

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"sync/atomic"
	"time"
)

var ErrStopIteration = errors.New("stop iteration")

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence

	isRunning *atomic.Bool
	stopChan  chan struct{}

	runF func(opt badger.Options) (*badger.DB, error)
	opts badger.Options
}

func New(
	path string,
	isInMemory bool,
	logLvl string,
) *DB {
	opts := badger.
		DefaultOptions(path + "/storage").
		WithSyncWrites(false).
		WithIndexCacheSize(256 << 20).
		WithCompression(options.Snappy).
		WithNumCompactors(2).
		WithLogger(nil)

	if isInMemory {
		opts.WithDir("").WithValueDir("").WithInMemory(isInMemory)
	}

	storage := &DB{
		badger: nil, stopChan: make(chan struct{}), isRunning: new(atomic.Bool),
		sequence: nil, runF: badger.Open, opts: opts,
	}

	return storage
}

func (db *DB) Run(password []byte) (err error) {
	if db.isRunning.Load() {
		return nil
	}
	if len(password) != 0 {
		db.opts.EncryptionKey = password
	}

	db.badger, err = db.runF(db.opts)
	if err != nil {
		return err
	}

	db.isRunning.Store(true)
	fmt.Println("DATABASE IS RUNNING!")
	db.sequence, err = db.badger.GetSequence([]byte("SEQUENCE:unified"), 100)
	if err != nil {
		return err
	}
	go db.runEventualGC()
	return nil
}

func (db *DB) runEventualGC() {
	fmt.Println("badger GC started")
	db.badger.RunValueLogGC(0.5)
	for {
		select {
		case <-time.After(time.Hour * 24):
			for {
				err := db.badger.RunValueLogGC(0.5)
				if err == badger.ErrNoRewrite {
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
		return errors.New("db is not running")
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

type IterKeysValuesFunc func(key string, val []byte) error

func (db *DB) IterateKeysValues(prefix string, handler IterKeysValuesFunc) error {
	if !db.isRunning.Load() {
		return errors.New("db is not running")
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
		return errors.New("db is not running")
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), value)
		return txn.SetEntry(e)
	})
}

func (db *DB) Get(key string) ([]byte, error) {
	if !db.isRunning.Load() {
		return nil, errors.New("db is not running")
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
		return errors.New("db is not running")
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
		return errors.New("db is not running")
	}
	return db.badger.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (db *DB) NextSequence() (uint64, error) {
	if !db.isRunning.Load() {
		return 0, errors.New("db is not running")
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
		if err == badger.ErrNoRewrite {
			return
		}
	}
}

func (db *DB) Close() error {
	close(db.stopChan)
	if db.sequence != nil {
		db.sequence.Release()
	}
	if db.badger == nil {
		return nil
	}
	if !db.isRunning.Load() {
		return nil
	}
	db.isRunning.Store(false)
	return db.badger.Close()
}
