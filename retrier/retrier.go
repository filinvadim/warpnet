// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package retrier

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

type retrierError string

func (e retrierError) Error() string {
	return string(e)
}

const (
	ErrDeadlineReached retrierError = "retrier: deadline reached"
	ErrStopTrying      retrierError = "retrier: stop trying"
)

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

func (r *retrier) Try(ctx context.Context, f RetrierFunc) (err error) {
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

		err = f()
		if errors.Is(err, ErrStopTrying) {
			return err
		}
		if err == nil {
			return nil
		}

		switch r.backoff {
		case NoBackoff:

		case ArithmeticalBackoff:
			interval = interval + interval
		case ExponentialBackoff:
			interval = interval * interval
		}
		sleepDuration := interval + jitter(r.minInterval)
		time.Sleep(sleepDuration)
	}

	return fmt.Errorf("%v %w", err, ErrDeadlineReached)
}

func jitter(minInterval time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(minInterval / 2))) // Add jitter
}
