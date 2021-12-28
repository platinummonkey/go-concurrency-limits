package limiter

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type testLifoQueueContextKey int

func TestLifoQueue(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	q := lifoQueue{}

	asrt.Equal(uint64(0), q.len())
	_, ctx := q.peek()
	asrt.Equal(uint64(0), q.len())
	asrt.Nil(ctx)
	asrt.Nil(q.pop())

	ctx1 := context.WithValue(context.Background(), testLifoQueueContextKey(1), 1)
	q.push(ctx1)

	_, element := q.peek()
	asrt.Equal(uint64(1), q.len())
	asrt.NotNil(element.ctx)
	asrt.Equal(ctx1, element.ctx)

	// add a 2nd
	ctx2 := context.WithValue(context.Background(), testLifoQueueContextKey(2), 2)
	q.push(ctx2)

	// make sure it's still LIFO
	_, element = q.peek()
	asrt.Equal(uint64(2), q.len())
	asrt.NotNil(element.ctx)
	asrt.Equal(ctx2, element.ctx)
	asrt.Equal(ctx2, q.top.ctx)

	// pop off
	element = q.pop()
	asrt.NotNil(element)
	asrt.Equal(ctx2, element.ctx)

	// check that we only have one again
	_, element = q.peek()
	asrt.Equal(uint64(1), q.len())
	asrt.NotNil(element.ctx)
	asrt.Equal(ctx1, element.ctx)

	// add a 2nd & 3rd
	ctx3 := context.WithValue(context.Background(), testLifoQueueContextKey(3), 3)
	q.push(ctx3)
	ctx4 := context.WithValue(context.Background(), testLifoQueueContextKey(4), 4)
	q.push(ctx4)

}

func TestLifoQueue_Evict(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	q := lifoQueue{}

	asrt.Equal(uint64(0), q.len())
	_, e := q.peek()
	asrt.Equal(uint64(0), q.len())
	asrt.Nil(e)
	asrt.Nil(q.pop())

	var evictFunc []func()
	for i := 1; i <= 10; i++ {
		ctx := context.WithValue(context.Background(), testLifoQueueContextKey(1), i)
		e, _ := q.push(ctx)
		evictFunc = append(evictFunc, e)
	}

	// remove last
	evictFunc[0]()
	asrt.Equal(uint64(9), q.len())

	// remove first
	evictFunc[9]()
	asrt.Equal(uint64(8), q.len())

	// remove middle
	evictFunc[4]()
	asrt.Equal(uint64(7), q.len())

	seenElements := make(map[int]struct{}, q.len())
	var element *lifoElement
	for {
		element = q.pop()
		if element == nil {
			break
		}
		id := element.ctx.Value(testLifoQueueContextKey(1)).(int)
		_, seen := seenElements[id]
		asrt.False(seen, "no duplicate element ids allowed")
		seenElements[id] = struct{}{}
	}
	asrt.Equal(uint64(0), q.len())
	asrt.Equal(7, len(seenElements))

	q = lifoQueue{}
	ctx := context.WithValue(context.Background(), testLifoQueueContextKey(1), 1)
	evict, _ := q.push(ctx)

	// Remove very last item leaving queue empty
	evict()
	asrt.Equal(uint64(0), q.len())
}

func TestLifoBlockingListener(t *testing.T) {
	t.Parallel()
	delegateLimiter, _ := NewDefaultLimiterWithDefaults(
		"",
		strategy.NewSimpleStrategy(20),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	limiter := NewLifoBlockingLimiterWithDefaults(delegateLimiter)
	delegateListener, _ := delegateLimiter.Acquire(context.Background())
	listener := LifoBlockingListener{
		delegateListener: delegateListener,
		limiter:          limiter,
	}
	listener.OnSuccess()
	listener.OnIgnore()
	listener.OnDropped()
}

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
		asrt.True(strings.Contains(limiter.String(), "LifoBlockingLimiter{delegate=DefaultLimiter{"))
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
		asrt.True(strings.Contains(limiter.String(), "LifoBlockingLimiter{delegate=DefaultLimiter{"))
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

	t.Run("CtxCancelled", func(t2 *testing.T) {
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
		limiter := NewLifoBlockingLimiterFromConfig(
			delegateLimiter,
			LifoLimiterConfig{
				BacklogEvictDoneCtx: true,
				MaxBacklogTimeout:   1 * time.Hour,
			},
		)
		asrt.NotNil(limiter)

		// acquire all tokens first
		listeners := make([]core.Listener, 0)
		for i := 0; i < 10; i++ {
			listener, ok := limiter.Acquire(context.Background())
			asrt.True(ok)
			asrt.NotNil(listener)
			listeners = append(listeners, listener)
		}

		wg := sync.WaitGroup{}
		wg.Add(5)
		for i := 0; i < 5; i++ {
			go func(j int) {
				wg.Done()
				listener, ok := limiter.Acquire(context.Background())
				asrt.True(ok, "must be true for j %d", j)
				asrt.NotNil(listener, "must be not be nil for j %d", j)
			}(i)
		}

		cancelledCtx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, func() {
			cancel()
		})

		token, ok := limiter.Acquire(cancelledCtx)
		asrt.False(ok)
		asrt.Nil(token)

		asrt.Equal(limiter.backlog.len(), uint64(5))
	})
}
