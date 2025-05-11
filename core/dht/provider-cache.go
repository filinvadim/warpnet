package dht

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	log "github.com/sirupsen/logrus"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type ProviderCacheStorer interface {
	ListProviders() (_ map[string][]warpnet.PeerAddrInfo, err error)
	AddProvider(ctx context.Context, key []byte, prov warpnet.PeerAddrInfo) error
	GetProviders(ctx context.Context, key []byte) ([]warpnet.PeerAddrInfo, error)
	io.Closer
}

type addrEntry struct {
	addr   warpnet.PeerAddrInfo
	readAt time.Time
}

type ProviderCache struct {
	ctx      context.Context
	db       ProviderCacheStorer
	m        *sync.Map
	stopChan chan struct{}
	isClosed *atomic.Bool
}

func NewProviderCache(ctx context.Context, db ProviderCacheStorer) (*ProviderCache, error) {
	pc := &ProviderCache{
		ctx:      ctx,
		db:       db,
		m:        new(sync.Map),
		stopChan: make(chan struct{}),
		isClosed: new(atomic.Bool),
	}
	prestoredProviders, err := db.ListProviders()
	if err != nil {
		return nil, err
	}

	for key, addrs := range prestoredProviders {
		if addrs == nil {
			continue
		}

		entries := make([]addrEntry, 0, len(addrs))
		for _, addr := range addrs {
			entries = append(entries, addrEntry{addr, time.Now()})
		}
		pc.m.Store(key, entries)

	}
	go pc.dumpProviders()
	return pc, nil
}

func (d *ProviderCache) dumpProviders() {
	log.Infoln("dht: providers cache: is running")
	tick := time.NewTicker(time.Minute * 10)
	defer tick.Stop()
	for {
		if d.isClosed.Load() {
			return
		}
		now := time.Now()
		select {
		case <-tick.C:
			d.m.Range(func(key, values interface{}) bool {
				keyStr, ok := key.(string)
				if !ok {
					return true
				}
				entries, ok := values.([]addrEntry)
				if !ok {
					return true
				}

				var validEntriesForCache []addrEntry
				for _, v := range entries {
					if v.readAt.After(now.Add(-8 * time.Hour)) {
						validEntriesForCache = append(validEntriesForCache, v)
						continue
					}
					if err := d.db.AddProvider(d.ctx, []byte(keyStr), v.addr); err != nil {
						log.Errorf("dht: adding provider %s to db: %v", key, err)
					}
				}

				// Update only if something was removed
				if len(validEntriesForCache) < len(entries) {
					d.m.Store(keyStr, validEntriesForCache)
				}
				return true
			})

		case <-d.stopChan:
			return
		}
	}
}

func (d *ProviderCache) AddProvider(ctx context.Context, key []byte, prov warpnet.PeerAddrInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	value, ok := d.m.Load(string(key))
	if !ok {
		d.m.Store(string(key), []addrEntry{{prov, time.Now()}})
		return nil
	}
	entries, ok := value.([]addrEntry)
	if !ok {
		d.m.Store(string(key), []addrEntry{{prov, time.Now()}})
		return nil
	}
	entries = append(entries, addrEntry{prov, time.Now()})
	d.m.Store(string(key), entries)

	return nil
}

func (d *ProviderCache) GetProviders(ctx context.Context, key []byte) (_ []warpnet.PeerAddrInfo, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	value, valueOk := d.m.Load(string(key))
	entries, entriesOk := value.([]addrEntry)
	if !valueOk || !entriesOk {
		providers, err := d.db.GetProviders(ctx, key)
		if err != nil {
			return nil, err
		}

		entries := make([]addrEntry, 0, len(providers))
		for _, prov := range providers {
			entries = append(entries, addrEntry{prov, time.Now()})
		}
		d.m.Store(string(key), entries)
		return providers, nil
	}

	addrs := make([]warpnet.PeerAddrInfo, 0, len(entries))
	for i, entry := range entries {
		entries[i].readAt = time.Now()
		addrs = append(addrs, entry.addr)
	}
	d.m.Store(string(key), entries)

	return addrs, nil
}

func (d *ProviderCache) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("dht: prividers cache: recovered: %v", r)
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	if d.isClosed.Load() {
		return nil
	}

	d.m.Range(func(key, values interface{}) bool {
		keyStr, ok := key.(string)
		if !ok {
			return true
		}
		entries, ok := values.([]addrEntry)
		if !ok {
			return true
		}
		for _, v := range entries {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second/2)
			_ = d.db.AddProvider(ctx, []byte(keyStr), v.addr)
			cancel()
		}
		return true
	})

	d.isClosed.Store(true)
	log.Infoln("dht: providers cache: stopped")
	close(d.stopChan)
	return err
}
