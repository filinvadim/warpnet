package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/json"
	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
)

const (
	NodesNamespace        = "NODES"
	ProvidersSubNamespace = "PROVIDERS"
)

var (
	_ ds.Batching             = (*NodeRepo)(nil)
	_ providers.ProviderStore = (*NodeRepo)(nil)
)

type NodeRepo struct {
	db *storage.DB

	stopChan chan struct{}
}

// Implements the datastore.Batch interface, enabling batching support for
// the badger Datastore.
type batch struct {
	ds         *NodeRepo
	writeBatch *badger.WriteBatch
}

//	implicit bool - Whether this transaction has been implicitly created as a result of a direct Datastore
//	method invocation.

func NewNodeRepo(db *storage.DB) *NodeRepo {
	nr := &NodeRepo{
		db:       db,
		stopChan: make(chan struct{}),
	}

	return nr
}

func (d *NodeRepo) Put(ctx context.Context, key ds.Key, value []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return d.db.WriteTxn(func(tx *badger.Txn) error {
		return tx.Set(key.Bytes(), value)
	})
}

func (d *NodeRepo) Sync(ctx context.Context, _ ds.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return d.db.Sync()
}

func (d *NodeRepo) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return d.db.WriteTxn(func(tx *badger.Txn) error {
		return tx.SetEntry(badger.NewEntry(key.Bytes(), value).WithTTL(ttl))
	})
}

func (d *NodeRepo) SetTTL(ctx context.Context, key ds.Key, ttl time.Duration) error {
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
	if ctx.Err() != nil {
		return t, ctx.Err()
	}

	if d.db.IsClosed() {
		return t, storage.ErrNotRunning
	}

	expiration := time.Time{}
	err = d.db.ReadTxn(func(tx *badger.Txn) error {
		item, err := tx.Get(key.Bytes())
		if errors.Is(err, badger.ErrKeyNotFound) {
			return ds.ErrNotFound
		} else if err != nil {
			return err
		}
		expiration = time.Unix(int64(item.ExpiresAt()), 0)
		return nil
	})
	return expiration, err
}

func (d *NodeRepo) Get(ctx context.Context, key ds.Key) (value []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if d.db.IsClosed() {
		return nil, storage.ErrNotRunning
	}

	value = []byte{}
	err = d.db.ReadTxn(func(tx *badger.Txn) error {
		item, err := tx.Get(key.Bytes())
		if errors.Is(err, badger.ErrKeyNotFound) {
			err = ds.ErrNotFound
		}
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		return err
	})
	return value, err
}

func (d *NodeRepo) Has(ctx context.Context, key ds.Key) (_ bool, err error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if d.db.IsClosed() {
		return false, storage.ErrNotRunning
	}

	err = d.db.ReadTxn(func(tx *badger.Txn) error {
		_, err := tx.Get(key.Bytes())
		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			return nil
		case err == nil:
			return nil
		default:
			return fmt.Errorf("has: %w", err)
		}
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *NodeRepo) GetSize(ctx context.Context, key ds.Key) (_ int, err error) {
	size := -1

	if ctx.Err() != nil {
		return size, ctx.Err()
	}

	if d.db.IsClosed() {
		return size, storage.ErrNotRunning
	}

	err = d.db.ReadTxn(func(tx *badger.Txn) error {
		item, err := tx.Get(key.Bytes())
		switch {
		case err == nil:
			size = int(item.ValueSize())
			return nil
		case errors.Is(err, badger.ErrKeyNotFound):
			return ds.ErrNotFound
		default:
			return fmt.Errorf("size: %w", err)
		}
	})
	return size, err
}

func (d *NodeRepo) Delete(ctx context.Context, key ds.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if d.db.IsClosed() {
		return storage.ErrNotRunning
	}
	return d.db.WriteTxn(func(tx *badger.Txn) error {
		return tx.Delete(key.Bytes())
	})
}

// DiskUsage implements the PersistentDatastore interface.
// It returns the sum of lsm and value log files sizes in bytes.
func (d *NodeRepo) DiskUsage(ctx context.Context) (uint64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	if d.db.IsClosed() {
		return 0, storage.ErrNotRunning
	}
	lsm, vlog := d.db.InnerDB().Size()
	return uint64(lsm + vlog), nil
}

func (d *NodeRepo) Query(ctx context.Context, q dsq.Query) (res dsq.Results, err error) {
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

func (d *NodeRepo) query(tx *badger.Txn, q dsq.Query, implicit bool) (dsq.Results, error) {
	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = !q.KeysOnly

	prefix := ds.NewKey(q.Prefix).String()
	if prefix != "/" {
		opt.Prefix = []byte(prefix + "/")
	}

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
	return time.Unix(int64(item.ExpiresAt()), 0)
}

func (d *NodeRepo) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("close recovered: %v", r)
		}
	}()
	close(d.stopChan)
	return nil
}

// Batch creates a new Batch object. This provides a way to do many writes, when
// there may be too many to fit into a single transaction.
func (d *NodeRepo) Batch(ctx context.Context) (ds.Batch, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if d.db.IsClosed() {
		return nil, storage.ErrNotRunning
	}

	b := &batch{d, d.db.InnerDB().NewWriteBatch()}
	// Ensure that incomplete transaction resources are cleaned up in case
	// batch is abandoned.
	runtime.SetFinalizer(b, func(b *batch) {
		log.Printf(
			"batch not committed or canceled: %v",
			b.Cancel(),
		)
	})

	return b, nil
}

var _ ds.Batch = (*batch)(nil)

func (b *batch) Put(ctx context.Context, key ds.Key, value []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	return b.put(key, value)
}

func (b *batch) put(key ds.Key, value []byte) error {
	return b.writeBatch.Set(key.Bytes(), value)
}

func (b *batch) putWithTTL(key ds.Key, value []byte, ttl time.Duration) error {
	return b.writeBatch.SetEntry(badger.NewEntry(key.Bytes(), value).WithTTL(ttl))
}

func (b *batch) Delete(ctx context.Context, key ds.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	return b.writeBatch.Delete(key.Bytes())
}

func (b *batch) Commit(_ context.Context) error {
	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	err := b.writeBatch.Flush()
	if err != nil {
		// Discard incomplete transaction held by b.writeBatch
		b.Cancel()
		return err
	}
	runtime.SetFinalizer(b, nil)
	return nil
}

func (b *batch) Cancel() error {
	if b.ds.db.IsClosed() {
		return storage.ErrNotRunning
	}

	b.writeBatch.Cancel()
	runtime.SetFinalizer(b, nil)
	return nil
}

func (d *NodeRepo) AddProvider(ctx context.Context, key []byte, prov peer.AddrInfo) error {
	addrs, err := d.GetProviders(ctx, key)
	if err != nil {
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
	providersKey := storage.NewPrefixBuilder(NodesNamespace).
		AddSubPrefix(ProvidersSubNamespace).
		Build()
	items, _, err := d.db.List(providersKey, nil, nil)
	if err != nil {
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
