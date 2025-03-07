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
	_           [56]byte // false sharing prevention
	hasFailed   atomic.Bool
	minInterval time.Duration
	_           [56]byte
}

// New creates a retrier
func New(minInterval time.Duration) Retrier {
	return &retrier{
		mux:         &sync.Mutex{},
		hasFailed:   atomic.Bool{},
		minInterval: minInterval,
	}
}

func (r *retrier) Try(f func() (bool, error), deadline time.Time) error {
	if hasFailed := r.hasFailed.Load(); hasFailed {
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
