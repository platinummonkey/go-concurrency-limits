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

func BenchmarkBlockingLimiter(b *testing.B) {

	benchLimiter := func(b *testing.B, limitCount, acquireCount int, timeout time.Duration) {
		b.StopTimer()

		l := limit.NewSettableLimit("test", limitCount, nil)
		defaultLimiter, err := NewDefaultLimiter(
			l,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(limitCount),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		if err != nil {
			b.Fatal(err.Error())
		}

		blockingLimiter := NewBlockingLimiter(defaultLimiter, timeout, limit.NoopLimitLogger{})

		wg := sync.WaitGroup{}
		wg.Add(acquireCount)

		startCount := sync.WaitGroup{}
		startCount.Add(acquireCount)

		released := make(chan int, acquireCount)
		startAcquire := make(chan struct{})
		for i := 0; i < acquireCount; i++ {
			go func(j int) {
				startCount.Done()
				defer wg.Done()
				<-startAcquire

				listener, ok := blockingLimiter.Acquire(context.Background())
				if ok && listener != nil {
					listener.OnSuccess()
					released <- 1
					return
				}
				released <- 0
			}(i)
		}

		startCount.Wait()
		b.StartTimer()
		close(startAcquire)

		wg.Wait()

		b.StopTimer()
		sumReleased := 0
		for i := 0; i < acquireCount; i++ {
			sumReleased += <-released
		}
		if sumReleased != acquireCount {
			b.Fatal("not enough released")
		}
		b.StartTimer()
	}

	b.Run("limiter_contention", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchLimiter(b, 2, 10, 0)
		}
	})

	b.Run("limiter_no_contention", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchLimiter(b, 100, 10, 0)
		}
	})

	b.Run("limiter_contention_w_timeout", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchLimiter(b, 2, 10, 10*time.Minute)
		}
	})

	b.Run("limiter_no_contention_w_timeout", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchLimiter(b, 100, 10, 10*time.Minute)
		}
	})

}

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
				ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), testContextKey, j), time.Millisecond*400)
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
		asrt.InDelta(4, sumReleased, 1.0, "expected roughly half to succeed")
	})
}
