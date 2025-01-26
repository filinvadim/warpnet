package retrier

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDeadline(t *testing.T) {
	duration := 1 * time.Second
	r := New(duration)
	attemptsChan := make(chan struct{}, 10)
	r.Try(func() (bool, error) {
		t.Log("tryout")
		attemptsChan <- struct{}{}
		return false, nil
	}, time.Now().Add(5*duration))
	assert.NotEqual(t, 1, len(attemptsChan))
	assert.NotEqual(t, 0, len(attemptsChan))
}
