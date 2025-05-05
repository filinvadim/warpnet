package discovery

import (
	"sync"
	"time"
)

type leakyBucketRateLimiter struct {
	capacity     int
	remaining    int
	leakInterval time.Duration
	lastLeak     time.Time

	mu *sync.Mutex
}

func newRateLimiter(capacity int, leakPer10Sec int) *leakyBucketRateLimiter {
	return &leakyBucketRateLimiter{
		capacity:     capacity,
		remaining:    0,
		leakInterval: (time.Second * 10) / time.Duration(leakPer10Sec),
		lastLeak:     time.Now(),
		mu:           new(sync.Mutex),
	}
}

func (b *leakyBucketRateLimiter) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastLeak)
	leaks := int(elapsed / b.leakInterval)
	if leaks > 0 {
		b.remaining -= leaks
		if b.remaining < 0 {
			b.remaining = 0
		}
		b.lastLeak = now
	}

	if b.remaining < b.capacity {
		b.remaining++
		return true
	}

	return false
}
