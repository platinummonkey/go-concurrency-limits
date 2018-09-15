package limiter

import (
	"context"
	"sync"
	"testing"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
	"github.com/stretchr/testify/assert"
)

func TestBlockingLimiter(t *testing.T) {
	t.Run("Unblocked", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit(10)
		noopLogger := limit.NoopLimitLogger{}
		defaultLimiter, err := NewDefaultLimiter(
			l,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(10),
			noopLogger,
			core.EmptyMetricRegistryInstance,
		)
		if !asrt.NoError(err) {
			asrt.FailNow("")
		}
		asrt.NotNil(defaultLimiter)
		blockingLimiter := NewBlockingLimiter(defaultLimiter)

		var listeners []core.Listener
		for i := 0; i < 10; i++ {
			listener, ok := blockingLimiter.Acquire(context.Background())
			if ok && listener != nil {
				listeners = append(listeners, listener)
			}
		}

		l.SetLimit(1)

		for _, listener := range listeners {
			listener.OnSuccess()
		}

		blockingLimiter.Acquire(nil)
	})

	t.Run("MultipleBlocked", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit(1)
		noopLogger := limit.NoopLimitLogger{}
		defaultLimiter, err := NewDefaultLimiter(
			l,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(1),
			noopLogger,
			core.EmptyMetricRegistryInstance,
		)
		if !asrt.NoError(err) {
			asrt.FailNow("")
		}
		asrt.NotNil(defaultLimiter)
		blockingLimiter := NewBlockingLimiter(defaultLimiter)

		wg := sync.WaitGroup{}
		wg.Add(8)

		released := make(chan int, 8)

		for i := 0; i < 8; i++ {
			go func(j int) {
				defer wg.Done()
				listener, ok := blockingLimiter.Acquire(context.Background())
				if ok && listener != nil {
					listener.OnSuccess()
					released <- 1
					return
				}
				released <- 0
			}(i)
		}

		wg.Wait()

		sumReleased := 0
		for i := 0; i < 8; i++ {
			sumReleased += <-released
		}
		asrt.Equal(8, sumReleased)
	})
}
