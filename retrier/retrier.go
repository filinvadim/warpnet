package retrier

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrDeadlineReached = errors.New("deadline reached")

type Retrier interface {
	Try(f func() (bool, error), deadline time.Time) error
}

type retrier struct {
	mux         *sync.Mutex
	hasFailed   atomic.Value
	minInterval time.Duration
}

// New creates a retrier
func New(minInterval time.Duration) Retrier {
	return &retrier{
		mux:         &sync.Mutex{},
		hasFailed:   atomic.Value{},
		minInterval: minInterval,
	}
}

func (r *retrier) Try(f func() (bool, error), deadline time.Time) error {
	if hasFailed, ok := r.hasFailed.Load().(bool); ok && hasFailed {
		r.mux.Lock()
		defer r.mux.Unlock()
	}

	lastInterval := r.minInterval

	var now = time.Now()

	for now.Before(deadline) {
		if successfullAttempt, err := f(); successfullAttempt || err != nil {
			r.hasFailed.Store(false)
			return err
		}
		r.hasFailed.Store(true)
		time.Sleep(lastInterval)
		now = time.Now()
	}

	return nil
}
