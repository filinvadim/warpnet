package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"slices"
	"sync"
	"time"
)

type addrEntry struct {
	addr   types.PeerAddrInfo
	readAt time.Time
}

type ProviderCache struct {
	ctx      context.Context
	db       PersistentLayer
	mutex    *sync.RWMutex
	m        map[string][]addrEntry
	stopChan chan struct{}
}

func NewProviderCache(ctx context.Context, db PersistentLayer) (*ProviderCache, error) {
	pc := &ProviderCache{
		ctx:      ctx,
		db:       db,
		mutex:    new(sync.RWMutex),
		m:        make(map[string][]addrEntry),
		stopChan: make(chan struct{}),
	}
	prestoredProviders, err := db.ListProviders()
	if err != nil {
		return nil, err
	}

	for key, addrs := range prestoredProviders {
		if addrs == nil {
			pc.m[key] = make([]addrEntry, len(addrs))
		}
		for _, addr := range addrs {
			pc.m[key] = append(pc.m[key], addrEntry{addr, time.Now()})
		}

	}
	go pc.dumpProviders()
	return pc, nil
}

func (d *ProviderCache) dumpProviders() {
	log.Println("providers cache is running")
	tick := time.NewTicker(time.Minute * 10)
	defer tick.Stop()
	for {
		now := time.Now()
		select {
		case <-tick.C:
			d.mutex.RLock()
			for key, values := range d.m {
				for i, v := range values {
					newValues := values
					if v.readAt.Before(now.Add(-time.Hour*24)) && i < len(newValues)-1 {
						slices.Delete(newValues, i, i+1)
						delete(d.m, key)
						d.m[key] = newValues
						continue
					}
					if err := d.db.AddProvider(d.ctx, []byte(key), v.addr); err != nil {
						log.Printf("error adding provider %s to cache: %v", key, err)
					}
				}
			}
			d.mutex.RUnlock()
		case <-d.stopChan:
			return
		}
	}
}

func (d *ProviderCache) AddProvider(ctx context.Context, key []byte, prov peer.AddrInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	entries, ok := d.m[string(key)]
	if ok {
		entries = append(entries, addrEntry{prov, time.Now()})
		d.m[string(key)] = entries
		return nil
	}
	d.m[string(key)] = []addrEntry{{prov, time.Now()}}
	return nil
}

func (d *ProviderCache) GetProviders(ctx context.Context, key []byte) (addrs []peer.AddrInfo, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	d.mutex.RLock()
	entries, ok := d.m[string(key)]
	if ok {
		for _, entry := range entries {
			entry.readAt = time.Now()
		}
		d.m[string(key)] = entries
	}
	d.mutex.RUnlock()
	if ok {
		return addrs, nil
	}

	addrs, err = d.db.GetProviders(ctx, key)
	if err != nil {
		return addrs, err
	}
	if len(addrs) == 0 {
		return addrs, nil
	}

	entries = make([]addrEntry, 0, len(addrs))
	for _, addr := range addrs {
		entries = append(entries, addrEntry{addr, time.Now()})
	}

	d.mutex.Lock()
	d.m[string(key)] = entries
	d.mutex.Unlock()

	return addrs, nil
}

func (d *ProviderCache) Close() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	close(d.stopChan)
	return err
}
