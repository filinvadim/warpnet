/*

 Warpnet - Decentralized Social Network
 Copyright (C) 2025 Vadim Filin, https://github.com/filinvadim,
 <github.com.mecdy@passmail.net>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/peer"
	log "github.com/sirupsen/logrus"
	"math"
	"runtime"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
)

// slash is required because of: invalid datastore key: NODES:/peers/keys/AASAQAISEAXNRKHMX2O3AA26JM7NGIWUPOGIITJ2UHHXGX4OWIEKPNAW6YCSK/priv
const (
	NodesNamespace        = "/NODES"
	ProvidersSubNamespace = "PROVIDERS"
	BlocklistSubNamespace = "BLOCKLIST"
	InfoSubNamespace      = "INFO"
)

var (
	_              ds.Batching             = (*NodeRepo)(nil)
	_              providers.ProviderStore = (*NodeRepo)(nil)
	ErrNilNodeRepo                         = errors.New("node repo is nil")
)

type NodeStorer interface {
	NewWriteTxn() (storage.WarpTxWriter, error)
	NewReadTxn() (storage.WarpTxReader, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	GetExpiration(key storage.DatabaseKey) (uint64, error)
	GetSize(key storage.DatabaseKey) (int64, error)
	Sync() error
	IsClosed() bool
	InnerDB() *storage.WarpDB
	SetWithTTL(key storage.DatabaseKey, value []byte, ttl time.Duration) error
	Set(key storage.DatabaseKey, value []byte) error
	Delete(key storage.DatabaseKey) error
}

type NodeRepo struct {
	db NodeStorer

	stopChan chan struct{}
}

// Implements the datastore.Batch interface, enabling batching support for
// the badger Datastore.
type batch struct {
	ds         *NodeRepo
	writeBatch *badger.WriteBatch
}

func NewNodeRepo(db NodeStorer) *NodeRepo {
	nr := &NodeRepo{
		db:       db,
		stopChan: make(chan struct{}),
	}

	return nr
}

func (d *NodeRepo) Put(ctx context.Context, key ds.Key, value []byte) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()
	return d.db.Set(prefix, value)
}

func (d *NodeRepo) Sync(ctx context.Context, _ ds.Key) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return d.db.Sync()
}

func (d *NodeRepo) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	return d.db.SetWithTTL(prefix, value, ttl)

}

func (d *NodeRepo) SetTTL(ctx context.Context, key ds.Key, ttl time.Duration) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if d.db.IsClosed() {
		return storage.ErrNotRunning
	}

	item, err := d.Get(ctx, key)
	if err != nil {
		return err
	}
	return d.PutWithTTL(ctx, key, item, ttl)
}

func (d *NodeRepo) GetExpiration(ctx context.Context, key ds.Key) (t time.Time, err error) {
	if d == nil {
		return t, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return t, ctx.Err()
	}

	if d.db.IsClosed() {
		return t, storage.ErrNotRunning
	}

	expiration := time.Time{}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	expiresAt, err := d.db.GetExpiration(prefix)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return t, ds.ErrNotFound
	} else if err != nil {
		return t, err
	}

	if expiresAt > math.MaxInt64 {
		expiresAt = math.MaxInt64
	}
	expiration = time.Unix(int64(expiresAt), 0) //#nosec

	return expiration, err
}

func (d *NodeRepo) Get(ctx context.Context, key ds.Key) (value []byte, err error) {
	if d == nil {
		return nil, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if d.db.IsClosed() {
		return nil, storage.ErrNotRunning
	}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	value, err = d.db.Get(prefix)
	if errors.Is(err, badger.ErrKeyNotFound) {
		err = ds.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return value, err
}

func (d *NodeRepo) Has(ctx context.Context, key ds.Key) (_ bool, err error) {
	if d == nil {
		return false, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if d.db.IsClosed() {
		return false, storage.ErrNotRunning
	}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	_, err = d.db.Get(prefix)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound) || errors.Is(err, ds.ErrNotFound):
		return false, nil
	case err == nil:
		return true, nil
	default:
		return false, fmt.Errorf("has: %w", err)
	}
}

func (d *NodeRepo) GetSize(ctx context.Context, key ds.Key) (_ int, err error) {
	if d == nil {
		return -1, ErrNilNodeRepo
	}
	size := -1

	if ctx.Err() != nil {
		return size, ctx.Err()
	}

	if d.db.IsClosed() {
		return size, storage.ErrNotRunning
	}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	itemSize, err := d.db.GetSize(prefix)
	switch {
	case err == nil:
		return int(itemSize), nil
	case errors.Is(err, badger.ErrKeyNotFound):
		return 0, ds.ErrNotFound
	default:
		return 0, fmt.Errorf("size: %w", err)
	}
}

func (d *NodeRepo) Delete(ctx context.Context, key ds.Key) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if d.db.IsClosed() {
		return storage.ErrNotRunning
	}

	rootKey := buildRootKey(key)

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()

	return d.db.Delete(prefix)
}

// DiskUsage implements the PersistentDatastore interface.
// It returns the sum of lsm and value log files sizes in bytes.
func (d *NodeRepo) DiskUsage(ctx context.Context) (uint64, error) {
	if d == nil {
		return 0, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	if d.db.IsClosed() {
		return 0, storage.ErrNotRunning
	}
	lsm, vlog := d.db.InnerDB().Size()
	if (lsm + vlog) < 0 {
		return 0, errors.New("disk usage: malformed value")
	}
	return uint64(lsm + vlog), nil //#nosec
}

func (d *NodeRepo) Query(ctx context.Context, q dsq.Query) (res dsq.Results, err error) {
	if d == nil {
		return nil, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if d.db.IsClosed() {
		return nil, storage.ErrNotRunning
	}

	// We cannot defer txn.Discard() here, as the txn must remain active while the iterator is open.
	// https://github.com/dgraph-io/badger/commit/b1ad1e93e483bbfef123793ceedc9a7e34b09f79
	// The closing logic in the query goprocess takes care of discarding the implicit transaction.
	tx := d.db.InnerDB().NewTransaction(true)
	return d.query(tx, q, true)
}

func (d *NodeRepo) query(tx *storage.WarpTxn, q dsq.Query, implicit bool) (dsq.Results, error) {
	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = !q.KeysOnly

	key := ds.NewKey(q.Prefix).String()
	key = strings.TrimPrefix(key, "/")

	prefix := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(key).
		Build().
		Bytes()

	opt.Prefix = prefix

	// Handle ordering
	if len(q.Orders) > 0 {
		switch q.Orders[0].(type) {
		case dsq.OrderByKey, *dsq.OrderByKey:
		// We order by key by default.
		case dsq.OrderByKeyDescending, *dsq.OrderByKeyDescending:
			// Reverse order by key
			opt.Reverse = true
		default:
			// Ok, we have a weird order we can't handle. Let's
			// perform the _base_ query (prefix, filter, etc.), then
			// handle sort/offset/limit later.

			// Skip the stuff we can't apply.
			baseQuery := q
			baseQuery.Limit = 0
			baseQuery.Offset = 0
			baseQuery.Orders = nil

			// perform the base query.
			res, err := d.query(tx, baseQuery, implicit)
			if err != nil {
				return nil, err
			}

			// fix the query
			res = dsq.ResultsReplaceQuery(res, q)

			// Remove the parts we've already applied.
			naiveQuery := q
			naiveQuery.Prefix = ""
			naiveQuery.Filters = nil

			// Apply the rest of the query
			return dsq.NaiveQueryApply(naiveQuery, res), nil
		}
	}

	it := tx.NewIterator(opt)
	qrb := dsq.NewResultBuilder(q)
	qrb.Process.Go(func(worker goprocess.Process) {
		closedEarly := false
		defer func() {
			if closedEarly {
				select {
				case qrb.Output <- dsq.Result{
					Error: errors.New("core repo closed"),
				}:
				case <-qrb.Process.Closing():
				}
			}

		}()
		if d.db.IsClosed() {
			closedEarly = true
			return
		}

		// this iterator is part of an implicit transaction, so when
		// we're done we must discard the transaction. It's safe to
		// discard the txn it because it contains the iterator only.
		if implicit {
			defer tx.Discard()
		}
		defer it.Close()
		// All iterators must be started by rewinding.
		it.Rewind()

		// skip to the offset
		for skipped := 0; skipped < q.Offset && it.Valid(); it.Next() {
			// On the happy path, we have no filters and we can go
			// on our way.
			if len(q.Filters) == 0 {
				skipped++
				continue
			}

			// On the sad path, we need to apply filters before
			// counting the item as "skipped" as the offset comes
			// _after_ the filter.
			item := it.Item()

			matches := true
			check := func(value []byte) error {
				e := dsq.Entry{
					Key:   string(item.Key()),
					Value: value,
					Size:  int(item.ValueSize()), // this function is basically free
				}

				// Only calculate expirations if we need them.
				if q.ReturnExpirations {
					e.Expiration = expires(item)
				}
				matches = filter(q.Filters, e)
				return nil
			}

			// Maybe check with the value, only if we need it.
			var err error
			if q.KeysOnly {
				err = check(nil)
			} else {
				err = item.Value(check)
			}

			if err != nil {
				select {
				case qrb.Output <- dsq.Result{Error: err}:
				case <-d.stopChan:
					closedEarly = true
					return
				case <-worker.Closing(): // client told us to close early
					return
				}
			}
			if !matches {
				skipped++
			}
		}

		for sent := 0; (q.Limit <= 0 || sent < q.Limit) && it.Valid(); it.Next() {
			item := it.Item()
			e := dsq.Entry{Key: string(item.Key())}

			// Maybe get the value
			var result dsq.Result
			if !q.KeysOnly {
				b, err := item.ValueCopy(nil)
				if err != nil {
					result = dsq.Result{Error: err}
				} else {
					e.Value = b
					e.Size = len(b)
					result = dsq.Result{Entry: e}
				}
			} else {
				e.Size = int(item.ValueSize())
				result = dsq.Result{Entry: e}
			}

			if q.ReturnExpirations {
				result.Expiration = expires(item)
			}

			// Finally, filter it (unless we're dealing with an error).
			if result.Error == nil && filter(q.Filters, e) {
				continue
			}

			select {
			case qrb.Output <- result:
				sent++
			case <-d.stopChan:
				closedEarly = true
				return
			case <-worker.Closing(): // client told us to close early
				return
			}
		}
	})

	go func() {
		_ = qrb.Process.CloseAfterChildren()
	}()

	return qrb.Results(), nil
}

// filter returns _true_ if we should filter (skip) the entry
func filter(filters []dsq.Filter, entry dsq.Entry) bool {
	for _, f := range filters {
		if !f.Filter(entry) {
			return true
		}
	}
	return false
}

func expires(item *badger.Item) time.Time {
	expiresAt := item.ExpiresAt()
	if expiresAt > math.MaxInt64 {
		expiresAt--
	}
	return time.Unix(int64(expiresAt), 0) //#nosec
}

func (d *NodeRepo) Close() (err error) {
	if d == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("close recovered: %v", r)
		}
	}()
	close(d.stopChan)

	log.Infoln("node repo: query interrupted")
	return nil
}

// Batch creates a new Batch object. This provides a way to do many writes, when
// there may be too many to fit into a single transaction.
func (d *NodeRepo) Batch(ctx context.Context) (ds.Batch, error) {
	if d == nil {
		return nil, ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if d.db.IsClosed() {
		return nil, storage.ErrNotRunning
	}

	b := &batch{d, d.db.InnerDB().NewWriteBatch()}
	// Ensure that incomplete transaction resources are cleaned up in case
	// batch is abandoned.
	runtime.SetFinalizer(b, func(b *batch) { _ = b.Cancel() })

	return b, nil
}

var _ ds.Batch = (*batch)(nil)

func (b *batch) Put(ctx context.Context, key ds.Key, value []byte) error {
	if b == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	return b.put(key, value)
}

func (b *batch) put(key ds.Key, value []byte) error {
	if b == nil {
		return ErrNilNodeRepo
	}

	rootKey := buildRootKey(key)

	batchKey := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()
	return b.writeBatch.Set(batchKey.Bytes(), value)
}

func (b *batch) putWithTTL(key ds.Key, value []byte, ttl time.Duration) error {
	if b == nil {
		return ErrNilNodeRepo
	}

	rootKey := buildRootKey(key)

	batchKey := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()
	return b.writeBatch.SetEntry(badger.NewEntry(batchKey.Bytes(), value).WithTTL(ttl))
}

func (b *batch) Delete(ctx context.Context, key ds.Key) error {
	if b == nil {
		return ErrNilNodeRepo
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	rootKey := buildRootKey(key)

	batchKey := storage.NewPrefixBuilder(NodesNamespace).
		AddRootID(rootKey).
		Build()
	return b.writeBatch.Delete(batchKey.Bytes())
}

func (b *batch) Commit(_ context.Context) error {
	if b == nil {
		return ErrNilNodeRepo
	}
	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	err := b.writeBatch.Flush()
	if err != nil {
		// Discard incomplete transaction held by b.writeBatch
		_ = b.Cancel()
		return err
	}
	runtime.SetFinalizer(b, nil)
	return nil
}

func (b *batch) Cancel() error {
	if b == nil {
		return ErrNilNodeRepo
	}
	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	b.writeBatch.Cancel()
	runtime.SetFinalizer(b, nil)
	return nil
}

func (d *NodeRepo) AddProvider(ctx context.Context, key []byte, prov peer.AddrInfo) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	addrs, err := d.GetProviders(ctx, key)
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		return err
	}

	addrs = append(addrs, prov)
	bt, err := json.JSON.Marshal(addrs)
	if err != nil {
		return err
	}

	providerKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(ProvidersSubNamespace).
		AddRootID(string(key)).
		Build()

	return d.db.SetWithTTL(providerKey, bt, time.Hour*24*7)
}

func (d *NodeRepo) GetProviders(_ context.Context, key []byte) (addrs []peer.AddrInfo, err error) {
	if d == nil {
		return nil, ErrNilNodeRepo
	}
	providerKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(ProvidersSubNamespace).
		AddRootID(string(key)).
		Build()
	bt, err := d.db.Get(providerKey)
	if err != nil {
		return nil, err
	}

	err = json.JSON.Unmarshal(bt, &addrs)
	return addrs, err
}

func (d *NodeRepo) ListProviders() (_ map[string][]peer.AddrInfo, err error) {
	if d == nil {
		return nil, ErrNilNodeRepo
	}
	providersKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(ProvidersSubNamespace).
		Build()

	txn, err := d.db.NewReadTxn()
	if err != nil {
		return nil, err
	}
	defer txn.Rollback()

	items, _, err := txn.List(providersKey, nil, nil)
	if err != nil {
		return nil, err
	}

	if err := txn.Commit(); err != nil {
		return nil, err
	}

	providersMap := make(map[string][]peer.AddrInfo, len(items))
	for _, item := range items {
		var addrs []peer.AddrInfo
		providerKey := strings.TrimPrefix(item.Key, providersKey.String()+":")
		if err := json.JSON.Unmarshal(item.Value, &addrs); err != nil {
			return nil, err
		}
		providersMap[providerKey] = addrs
	}

	return providersMap, nil
}

func (d *NodeRepo) Blocklist(ctx context.Context, peerId warpnet.WarpPeerID) error {
	if d == nil {
		return ErrNilNodeRepo
	}
	if peerId == "" {
		return errors.New("empty peer ID")
	}
	blocklistKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(BlocklistSubNamespace).
		AddRootID(peerId.String()).
		Build()

	return d.Put(ctx, ds.NewKey(blocklistKey.String()), []byte(peerId.String()))
}

func (d *NodeRepo) IsBlocklisted(ctx context.Context, peerId warpnet.WarpPeerID) (bool, error) {
	if d == nil {
		return false, ErrNilNodeRepo
	}
	if peerId == "" {
		return false, nil
	}
	blocklistKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(BlocklistSubNamespace).
		AddRootID(peerId.String()).
		Build()
	_, err := d.Get(ctx, ds.NewKey(blocklistKey.String()))

	if errors.Is(err, storage.ErrKeyNotFound) || errors.Is(err, ds.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *NodeRepo) BlocklistRemove(ctx context.Context, peerId warpnet.WarpPeerID) (err error) {
	if d == nil {
		return ErrNilNodeRepo
	}
	if peerId == "" {
		return errors.New("empty peer ID")
	}
	blocklistKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(BlocklistSubNamespace).
		AddRootID(peerId.String()).
		Build()

	err = d.Delete(ctx, ds.NewKey(blocklistKey.String()))
	if errors.Is(err, storage.ErrKeyNotFound) || errors.Is(err, ds.ErrNotFound) {
		return nil
	}
	return err
}

func buildRootKey(key ds.Key) string {
	rootKey := strings.TrimPrefix(key.String(), "/")
	if len(rootKey) == 0 {
		rootKey = key.String()
	}
	return rootKey
}
