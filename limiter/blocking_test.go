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

type testListener struct {
	successCount int
	ignoreCount  int
	dropCount    int
}

func (l *testListener) OnSuccess() {
	l.successCount++
}

func (l *testListener) OnIgnore() {
	l.ignoreCount++
}

func (l *testListener) OnDropped() {
	l.dropCount++
}

type contextKey string

var testContextKey contextKey = "jobID"

func TestBlockingLimiter(t *testing.T) {
	t.Run("Unblocked", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit("test", 10, nil)
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
		blockingLimiter := NewBlockingLimiter(defaultLimiter, 0, noopLogger)
		// stringer
		asrt.True(strings.Contains(blockingLimiter.String(), "BlockingLimiter{delegate=DefaultLimiter{"))

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

		blockingLimiter.Acquire(context.Background())
	})

	t.Run("MultipleBlocked", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit("test", 1, nil)
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
		blockingLimiter := NewBlockingLimiter(defaultLimiter, 0, noopLogger)

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

	t.Run("BlockingListener", func(t2 *testing.T) {
		asrt := assert.New(t2)
		delegateListener := testListener{}
		listener := NewBlockingListener(&delegateListener)
		listener.OnSuccess()
		asrt.Equal(1, delegateListener.successCount)
		listener.OnIgnore()
		asrt.Equal(1, delegateListener.ignoreCount)
		listener.OnDropped()
		asrt.Equal(1, delegateListener.dropCount)

	})

	t.Run("BlockingLimiterTimeout", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit("test", 1, nil)
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
		blockingLimiter := NewBlockingLimiter(defaultLimiter, time.Millisecond*25, noopLogger)

		wg := sync.WaitGroup{}
		wg.Add(8)

		released := make(chan int, 8)

		for i := 0; i < 8; i++ {
			go func(j int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), testContextKey, j), time.Millisecond*410)
				defer cancel()
				listener, ok := blockingLimiter.Acquire(ctx)
				if ok && listener != nil {
					time.Sleep(time.Millisecond * 100)
					listener.OnSuccess()
					released <- 1
					return
				}
				released <- 0
			}(i)
			time.Sleep(time.Nanosecond * 50)
		}

		wg.Wait()

		sumReleased := 0
		for i := 0; i < 8; i++ {
			sumReleased += <-released
		}
		// we only expect half of them to complete before their deadlines
		asrt.Equal(4, sumReleased)
	})
}
