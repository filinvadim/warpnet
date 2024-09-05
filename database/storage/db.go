package storage

import (
	"crypto/rand"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"log"
	"time"
)

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence
	stopChan chan struct{}
}

func New(
	login, password, path string,
	isEncrypted, isInMemory bool,
	logLvl string,
) *DB {
	opts := badger.
		DefaultOptions(path + "/storage").
		WithSyncWrites(false).
		WithIndexCacheSize(256 << 20).
		WithCompression(options.Snappy).
		WithNumCompactors(2)

	if isEncrypted {
		opts.EncryptionKey = generateEncryptionKey(login, password)
	}
	if isInMemory {
		opts.WithDir("").WithValueDir("").WithInMemory(isInMemory)
	}

	switch logLvl {
	case "info":
		opts.WithLoggingLevel(1)
	case "debug":
		opts.WithLoggingLevel(0)
	case "error":
		opts.WithLoggingLevel(3)
	default:
		opts.WithLoggingLevel(1)
	}

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}

	stopChan := make(chan struct{})
	seq, err := db.GetSequence([]byte("unified"), 100)
	if err != nil {
		log.Fatal(err)
	}

	storage := &DB{badger: db, stopChan: stopChan, sequence: seq}

	go storage.runEventualGC()

	return storage
}

func generateEncryptionKey(login, password string) []byte {
	var encryptionKey = make([]byte, 32)
	if login == "" || password == "" {
		if _, err := rand.Read(encryptionKey); err != nil {
			log.Fatal("error generating random key:", err)
		}
		return encryptionKey
	}
	creds := []byte(login + password)
	if len(creds) > 32 {
		log.Fatal("login and password combination is too long")
	}
	copy(encryptionKey[32-len(creds):], creds)
	return encryptionKey
}

func (db *DB) runEventualGC() {
	fmt.Println("badger GC started")
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
	return db.badger.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), value)
		return txn.SetEntry(e)
	})
}

func (db *DB) Get(key string) ([]byte, error) {
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
	return db.badger.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

func (db *DB) NextSequence() (uint64, error) {
	return db.sequence.Next()
}

func (db *DB) GC() {
	for {
		err := db.badger.RunValueLogGC(0.5)
		if err == badger.ErrNoRewrite {
			return
		}
	}
}

func (db *DB) Close() error {
	close(db.stopChan)
	db.sequence.Release()
	return db.badger.Close()
}
