package storage

import (
	"crypto/rand"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type DB struct {
	badger   *badger.DB
	sequence *badger.Sequence
	stopChan chan struct{}
}

func New(login, password string) *DB {
	opts := badger.
		DefaultOptions(getDBPath()).
		WithSyncWrites(false).
		WithLoggingLevel(badger.INFO).
		WithIndexCacheSize(256 << 20).
		WithEncryptionKey(generateEncryptionKey(login, password)).
		WithCompression(options.Snappy).
		WithNumCompactors(2)
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

func getDBPath() string {
	var dbPath string

	switch runtime.GOOS {
	case "windows":
		// %LOCALAPPDATA% Windows
		appData := os.Getenv("LOCALAPPDATA") // C:\Users\{username}\AppData\Local
		if appData == "" {
			log.Fatal("failed to get path to LOCALAPPDATA")
		}
		dbPath = filepath.Join(appData, "badgerdb", "data")

	case "darwin", "linux", "android":
		homeDir := os.TempDir()
		dbPath = filepath.Join(homeDir, ".badgerdb", "data")

	default:
		log.Fatal("unsupported OS")
	}

	err := os.MkdirAll(dbPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	return dbPath
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

func (db *DB) Delete(key string) error {
	return db.badger.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

type KV map[string]string

func (db *DB) Iterate(prefix string) (KV, error) {
	kv := make(KV)
	err := db.badger.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		p := []byte(prefix)
		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			item := it.Item()

			k := append([]byte{}, item.Key()...)
			err := item.Value(func(v []byte) error {
				kv[string(k)] = string(v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return kv, err
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
