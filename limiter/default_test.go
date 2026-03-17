package limiter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

func TestDefaultListener(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	inFlight := int64(3)
	releaseCount := int64(0)
	f := func() {
		releaseCount++
	}
	limiter, _ := NewDefaultLimiterWithDefaults(
		"",
		strategy.NewSimpleStrategy(10),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	limiter.sample = measurements.NewDefaultImmutableSampleWindow()
	listener := DefaultListener{
		currentMaxInFlight: 1,
		inFlight:           &inFlight,
		token:              core.NewAcquiredStrategyToken(1, f),
		startTime:          time.Now().Unix(),
		minRTTThreshold:    10,
		limiter:            limiter,
		nextUpdateTime:     time.Now().Add(time.Minute * 10).Unix(),
	}

	// On Success
	listener.OnSuccess()
	asrt.Equal(int64(2), inFlight)
	asrt.Equal(int64(1), releaseCount)

	// On Ignore
	listener.OnIgnore()
	asrt.Equal(int64(1), inFlight)
	asrt.Equal(int64(2), releaseCount)

	// On Dropped
	listener.OnDropped()
	asrt.Equal(int64(0), inFlight)
	asrt.Equal(int64(3), releaseCount)
}

func TestDefaultLimiter(t *testing.T) {
	t.Parallel()

	t.Run("NewDefaultLimiterWithDefaults", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiterWithDefaults("", strategy.NewSimpleStrategy(10), limit.NoopLimitLogger{}, core.EmptyMetricRegistryInstance)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(20, l.EstimatedLimit())
		asrt.True(strings.Contains(l.String(), "DefaultLimiter{RTTCandidate="))
	})

	t.Run("NewDefaultLimiter", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiter(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(10),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("NewDefaultLimiterWithFactory", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiterWithFactory(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategyFactory(),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())

		// Verify strategy was initialized with the limit algorithm's estimated limit:
		// should be able to acquire exactly 10 tokens, not more.
		listeners := make([]core.Listener, 0)
		for i := 0; i < 10; i++ {
			listener, ok := l.Acquire(context.Background())
			asrt.Truef(ok, "expected acquire #%d to succeed", i+1)
			asrt.NotNil(listener)
			listeners = append(listeners, listener)
		}
		listener, ok := l.Acquire(context.Background())
		asrt.False(ok, "expected acquire beyond limit to fail")
		asrt.Nil(listener)
		for _, listener = range listeners {
			listener.OnSuccess()
		}
	})

	t.Run("NewDefaultLimiterWithFactoryNilLimitError", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiterWithFactory(
			nil,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategyFactory(),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.Error(err)
		asrt.Nil(l)
	})

	t.Run("NewDefaultLimiterWithFactoryNilFactoryError", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiterWithFactory(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			nil,
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.Error(err)
		asrt.Nil(l)
	})

	t.Run("NewDefaultLimiterMismatchedInitialLimitIsReconciled", func(t2 *testing.T) {
		// Demonstrates that even with the old NewDefaultLimiter API, passing a strategy
		// with a mismatched initial limit (999) is immediately overridden by the limit
		// algorithm's EstimatedLimit (10) at construction time (see issue #176).
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiter(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(999), // mismatched initial limit
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())

		// Despite strategy being initialized with 999, enforce limit should be 10.
		listeners := make([]core.Listener, 0)
		for i := 0; i < 10; i++ {
			listener, ok := l.Acquire(context.Background())
			asrt.Truef(ok, "expected acquire #%d to succeed", i+1)
			asrt.NotNil(listener)
			listeners = append(listeners, listener)
		}
		listener, ok := l.Acquire(context.Background())
		asrt.False(ok, "expected acquire beyond limit to fail (strategy limit was overridden to 10, not 999)")
		asrt.Nil(listener)
		for _, listener = range listeners {
			listener.OnSuccess()
		}
	})

	t.Run("Acquire", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewDefaultLimiter(
			limit.NewFixedLimit("test", 10, nil),
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(10),
			limit.NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())

		listeners := make([]core.Listener, 0)

		// Acquire tokens
		for i := 0; i < 10; i++ {
			listener, ok := l.Acquire(context.Background())
			asrt.True(ok)
			asrt.NotNil(listener)
			listeners = append(listeners, listener)
		}

		// try to acquire one more
		listener, ok := l.Acquire(context.Background())
		asrt.False(ok)
		asrt.Nil(listener)

		// release all
		for _, listener = range listeners {
			listener.OnSuccess()
		}

		listener, ok = l.Acquire(context.Background())
		asrt.True(ok)
		asrt.NotNil(listener)
		listener.OnSuccess()
	})
}
