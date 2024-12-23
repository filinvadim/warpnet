package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
	"slices"
	"sync"
	"time"
)

type addrEntry struct {
	addr    peer.AddrInfo
	created time.Time
}

type ProviderCache struct {
	ctx      context.Context
	nodeRepo *database.NodeRepo
	mutex    *sync.RWMutex
	m        map[string][]addrEntry
	stopChan chan struct{}
}

func NewProviderCache(ctx context.Context, nodeRepo *database.NodeRepo) *ProviderCache {
	pc := &ProviderCache{
		ctx:      ctx,
		nodeRepo: nodeRepo,
		mutex:    new(sync.RWMutex),
		m:        make(map[string][]addrEntry),
		stopChan: make(chan struct{}),
	}
	go pc.dumpProviders()
	return pc
}

func (d *ProviderCache) dumpProviders() {
	tick := time.NewTicker(time.Minute * 10)
	for {
		now := time.Now()
		select {
		case <-tick.C:
			d.mutex.RLock()
			for key, values := range d.m {
				for i, v := range values {
					newValues := values
					if v.created.Before(now.Add(-time.Hour * 24)) {
						slices.Delete(newValues, i, i+1)
						delete(d.m, key)
						d.m[key] = newValues
						continue
					}
					if err := d.nodeRepo.AddProvider(d.ctx, []byte(key), v.addr); err != nil {
						log.Printf("error adding provider %s to cache: %v", key, err)
					}
				}
			}
			d.mutex.RUnlock()
		case <-d.stopChan:
			tick.Stop()
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
	d.mutex.RUnlock()
	if ok {
		return addrs, nil
	}

	addrs, err = d.nodeRepo.GetProviders(ctx, key)
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
