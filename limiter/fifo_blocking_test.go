package limiter

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

func TestFifoBlockingLimiter(t *testing.T) {
	t.Parallel()

	t.Run("NewFifoBlockingLimiterWithDefaults", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegateLimiter, _ := NewDefaultLimiterWithDefaults(
			"",
			strategy.NewSimpleStrategy(20),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		limiter := NewFifoBlockingLimiterWithDefaults(delegateLimiter)
		asrt.NotNil(limiter)
		asrt.True(strings.Contains(limiter.String(), "QueueBlockingLimiter{delegate=DefaultLimiter{"))
	})

	t.Run("NewFifoBlockingLimiter", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegateLimiter, _ := NewDefaultLimiterWithDefaults(
			"",
			strategy.NewSimpleStrategy(20),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		limiter := NewFifoBlockingLimiter(delegateLimiter, -1, 0)
		asrt.NotNil(limiter)
		asrt.True(strings.Contains(limiter.String(), "QueueBlockingLimiter{delegate=DefaultLimiter{"))
	})

	t.Run("Acquire", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegateLimiter, _ := NewDefaultLimiter(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(10),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		limiter := NewFifoBlockingLimiterWithDefaults(delegateLimiter)
		asrt.NotNil(limiter)

		// acquire all tokens first
		listeners := make([]core.Listener, 0)
		for i := 0; i < 10; i++ {
			listener, ok := limiter.Acquire(context.Background())
			asrt.True(ok)
			asrt.NotNil(listener)
			listeners = append(listeners, listener)
		}

		// queue up 10 more waiting
		waitingListeners := make([]acquiredListenerFifo, 0)
		mu := sync.Mutex{}
		startupReady := make(chan bool, 1)
		wg := sync.WaitGroup{}
		wg.Add(10)
		for i := 0; i < 10; i++ {
			if i > 0 {
				select {
				case <-startupReady:
					// proceed
				}
			}
			go func(j int) {
				startupReady <- true
				listener, ok := limiter.Acquire(context.Background())
				asrt.True(ok)
				asrt.NotNil(listener)
				mu.Lock()
				waitingListeners = append(waitingListeners, acquiredListenerFifo{id: j, listener: listener})
				mu.Unlock()
				wg.Done()
			}(i)
		}

		// release all other listeners, so we can continue
		for _, listener := range listeners {
			listener.OnSuccess()
		}

		// wait for others
		wg.Wait()

		// check all eventually required. Note: due to scheduling, it's not entirely LIFO as scheduling will allow
		// some non-determinism
		asrt.Len(waitingListeners, 10)
		// release all
		for _, acquired := range waitingListeners {
			if acquired.listener != nil {
				acquired.listener.OnSuccess()
			}
		}
	})
}
