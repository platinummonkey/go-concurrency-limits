package limiter

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type testFifoQueueContextKey int

func TestFifoQueue(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	q := fifoQueue{
		q: list.New(),
	}

	asrt.Equal(0, q.len())
	el := q.peek()
	asrt.Nil(el)
	asrt.Nil(q.pop())

	ctx1 := context.WithValue(context.Background(), testFifoQueueContextKey(1), 1)
	q.push(ctx1)

	el = q.peek()
	asrt.Equal(1, q.len())
	asrt.NotNil(el)
	asrt.Equal(ctx1, el.ctx)

	// add a 2nd
	ctx2 := context.WithValue(context.Background(), testFifoQueueContextKey(2), 2)
	q.push(ctx2)

	// make sure it's still FIFO
	el = q.peek()
	asrt.Equal(2, q.len())
	asrt.NotNil(el)
	asrt.Equal(ctx1, el.ctx)

	// pop off
	el = q.pop()
	asrt.NotNil(el)
	asrt.Equal(ctx1, el.ctx)

	// check that we only have one again
	el = q.peek()
	asrt.Equal(1, q.len())
	asrt.NotNil(el)
	asrt.Equal(ctx2, el.ctx)

	// add a 2nd & 3rd
	ctx3 := context.WithValue(context.Background(), testFifoQueueContextKey(3), 3)
	el3, _ := q.push(ctx3)
	ctx4 := context.WithValue(context.Background(), testFifoQueueContextKey(4), 4)
	q.push(ctx4)

	// remove the middle
	q.remove(el3)
	el = q.peek()
	asrt.Equal(2, q.len())
	asrt.NotNil(el)
	asrt.Equal(ctx2, el.ctx)
	asrt.Equal(ctx4, q.q.Back().Value.(fifoElement).ctx)
}

func TestFifoBlockingListener(t *testing.T) {
	t.Parallel()
	delegateLimiter, _ := NewDefaultLimiterWithDefaults(
		"",
		strategy.NewSimpleStrategy(20),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	limiter := NewFifoBlockingLimiterWithDefaults(delegateLimiter)
	delegateListener, _ := delegateLimiter.Acquire(context.Background())
	listener := FifoBlockingListener{
		delegateListener: delegateListener,
		limiter:          limiter,
	}
	listener.OnSuccess()
	listener.OnIgnore()
	listener.OnDropped()
}

type acquiredListenerFifo struct {
	id       int
	listener core.Listener
}

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
		asrt.True(strings.Contains(limiter.String(), "FifoBlockingLimiter{delegate=DefaultLimiter{"))
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
		asrt.True(strings.Contains(limiter.String(), "FifoBlockingLimiter{delegate=DefaultLimiter{"))
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
