package retrier

import (
	"context"
	"math/rand"
	"time"
)

type retrierError string

func (e retrierError) Error() string {
	return string(e)
}

const ErrDeadlineReached retrierError = "deadline reached"

type (
	backoff byte

	RetrierFunc = func() error

	Retrier interface {
		Try(ctx context.Context, f RetrierFunc) error
	}

	retrier struct {
		minInterval time.Duration
		maxAttempts uint32
		backoff     backoff
	}
)

const (
	NoBackoff backoff = iota
	ArithmeticalBackoff
	ExponentialBackoff
)

// New creates a retrier with the given minimum interval between attempts and maximum retries.
func New(minInterval time.Duration, maxAttempts uint32, b backoff) Retrier {
	return &retrier{
		minInterval: minInterval,
		maxAttempts: maxAttempts,
		backoff:     b,
	}
}

func (r *retrier) Try(ctx context.Context, f RetrierFunc) error {
	var (
		attempt  uint32
		interval = r.minInterval
	)

	for attempt = 0; attempt < r.maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := f(); err == nil {
			return nil
		}

		switch r.backoff {
		case NoBackoff:
			interval = interval
		case ArithmeticalBackoff:
			interval = interval + interval
		case ExponentialBackoff:
			interval = interval * interval
		}
		sleepDuration := interval + jitter(r.minInterval)
		time.Sleep(sleepDuration)
	}

	return ErrDeadlineReached
}

func jitter(minInterval time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(minInterval / 2))) // Add jitter
}
