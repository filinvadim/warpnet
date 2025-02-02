package storage

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/filinvadim/warpnet/core/encrypting"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

/*
  BadgerDB — это высокопроизводительная, встраиваемая key-value база данных,
написанная на Go и использующая LSM-деревья (Log-Structured Merge-Trees) для эффективного
хранения и обработки данных. Ориентирована на сценарии с высокой нагрузкой, требующие
минимальной задержки и высокой пропускной способности.
Основные характеристики:
    Встраиваемая: Работает в рамках приложения без необходимости запуска отдельного сервера.
    Key-Value хранилище: Позволяет хранить и получать данные по ключу, используя эффективные индексы.
    LSM-архитектура: Обеспечивает высокую скорость записи за счёт лог-структурированного хранения данных.
    Zero GC overhead: Минимизирует влияние сборки мусора за счёт прямой работы с mmap и byte slices.
    ACID-транзакции: Поддерживает транзакции с изоляцией snapshot isolation.
    Режим работы в памяти и на диске: Позволяет хранить данные в RAM или на SSD/HDD.
    Низкое потребление ресурсов: Подходит для embedded-систем и серверных приложений с ограниченной памятью.

BadgerDB используется в тех случаях, когда требуется:
    Высокая скорость работы: Он быстрее, чем традиционные дисковые базы данных (например, BoltDB) благодаря LSM-структуре.
    Встраиваемое хранилище: Нет необходимости поднимать отдельный сервер базы данных (в отличие от Redis, PostgreSQL и др.).
    Эффективная работа с потоковыми записями: Подходит для логов, кэшей, брокеров сообщений и других write-intensive задач.
    Поддержка транзакций: Можно безопасно выполнять несколько операций в рамках одной транзакции.
    Работа с большими объёмами данных: Поддерживает шардирование и offloading на диск, что полезно при обработке больших массивов данных.
    Гибкость: Можно легко интегрировать в распределённые системы и P2P-приложения.

BadgerDB особенно хорош для систем, где важны высокая скорость записи, низкие накладные расходы и возможность работы без внешнего сервера БД.
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
	if db.isRunning.Load() {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("database username or password is empty")
	}
	hashSum := encrypting.ConvertToSHA256([]byte(username + "@" + password))
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
	log.Println("database garbage collection started")
	_ = db.badger.RunValueLogGC(discardRatio)
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
		iterNum := uint64(0)
		for it.Seek(p); it.ValidForPrefix(p); it.Next() {
			p = prefix.Bytes() // starting point found

			item := it.Item()
			key := string(item.Key())

			if strings.Contains(key, FixedKey) {
				return nil
			}

			if iterNum > *limit {
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
		if iterNum < *limit {
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

type WarpTxn = badger.Txn

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

func (db *DB) BatchSet(data []ListItem) error {
	var (
		lastIndex int
		isTooBig  bool
	)
	err := db.WriteTxn(func(txn *badger.Txn) error {
		for i, item := range data {
			key := item.Key
			value := item.Value
			err := txn.Set([]byte(key), value)
			if errors.Is(err, badger.ErrTxnTooBig) {
				isTooBig = true
				lastIndex = i
				return nil // force commit in the middle of iteration
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if isTooBig {
		leftovers := data[lastIndex:]
		data = nil
		err = db.BatchSet(leftovers)
	}
	return err
}

func (db *DB) BatchGet(keys ...DatabaseKey) ([]ListItem, error) {
	result := make([]ListItem, 0, len(keys))

	err := db.ReadTxn(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			it := ListItem{
				Key:   key.String(),
				Value: val,
			}
			result = append(result, it)
		}
		return nil
	})
	return result, err
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
