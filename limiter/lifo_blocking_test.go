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

type acquiredListenerLifo struct {
	id       int
	listener core.Listener
}

func TestLifoBlockingLimiter(t *testing.T) {
	t.Parallel()

	t.Run("NewLifoBlockingLimiterWithDefaults", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegateLimiter, _ := NewDefaultLimiterWithDefaults(
			"",
			strategy.NewSimpleStrategy(20),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		limiter := NewLifoBlockingLimiterWithDefaults(delegateLimiter)
		asrt.NotNil(limiter)
		asrt.True(strings.Contains(limiter.String(), "QueueBlockingLimiter{delegate=DefaultLimiter{"))
	})

	t.Run("NewLifoBlockingLimiter", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegateLimiter, _ := NewDefaultLimiterWithDefaults(
			"",
			strategy.NewSimpleStrategy(20),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		limiter := NewLifoBlockingLimiter(delegateLimiter, -1, 0, nil)
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
		limiter := NewLifoBlockingLimiterWithDefaults(delegateLimiter)
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
		waitingListeners := make([]acquiredListenerLifo, 0)
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
				asrt.True(ok, "must be true for j %d", j)
				asrt.NotNil(listener, "must be not be nil for j %d", j)
				mu.Lock()
				waitingListeners = append(waitingListeners, acquiredListenerLifo{id: j, listener: listener})
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
