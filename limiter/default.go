package limiter

import (
	"context"
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
)

const (
	defaultMinWindowTime   = int64(1e9) // (1 s) nanoseconds
	defaultMaxWindowTime   = int64(1e9) // (1 s) nanoseconds
	defaultMinRTTThreshold = int64(1e5) // (100 ms) nanoseconds
	defaultWindowSize      = int(100)   // Minimum observed samples to filter out sample windows with not enough significant samples
)

// DefaultListener for
type DefaultListener struct {
	currentMaxInFlight int64
	inFlight           *int64
	token              core.StrategyToken
	startTime          int64
	minRTTThreshold    int64
	limiter            *DefaultLimiter
	nextUpdateTime     int64
}

func (l *DefaultListener) OnSuccess() {
	atomic.AddInt64(l.inFlight, -1)
	l.token.Release()
	endTime := time.Now().UnixNano()
	rtt := endTime - l.startTime

	if rtt < l.minRTTThreshold {
		return
	}
	_, current := l.limiter.updateAndGetSample(func(window measurements.ImmutableSampleWindow) measurements.ImmutableSampleWindow {
		return *(window.AddSample(rtt, int(l.currentMaxInFlight)))
	})

	if endTime > l.nextUpdateTime {
		// double check just to be sure
		l.limiter.mu.Lock()
		defer l.limiter.mu.Unlock()
		if endTime > l.limiter.nextUpdateTime {
			if l.limiter.isWindowReady(current) {
				l.limiter.sample = measurements.NewImmutableSampleWindow(0, 0, 0, 0, false)
				minWindowTime := current.CandidateRTTNanoseconds() * 2
				if l.limiter.minWindowTime > minWindowTime {
					minWindowTime = l.limiter.minWindowTime
				}
				minVal := l.limiter.maxWindowTime
				if minWindowTime < minVal {
					minVal = minWindowTime
				}
				l.limiter.nextUpdateTime = endTime + minVal
				l.limiter.limit.Update(&current)
				l.limiter.strategy.SetLimit(l.limiter.limit.EstimatedLimit())
			}
		}
	}

}

func (l *DefaultListener) OnIgnore() {
	atomic.AddInt64(l.inFlight, -1)
	l.token.Release()
}

func (l *DefaultListener) OnDropped() {
	atomic.AddInt64(l.inFlight, -1)
	l.token.Release()
	l.limiter.updateAndGetSample(func(window measurements.ImmutableSampleWindow) measurements.ImmutableSampleWindow {
		return *(window.AddDroppedSample(int(l.currentMaxInFlight)))
	})
}

// DefaultLimiter is a Limiter that combines a plugable limit algorithm and enforcement strategy to enforce concurrency
// limits to a fixed resource.
type DefaultLimiter struct {
	limit           core.Limit
	strategy        core.Strategy
	minWindowTime   int64
	maxWindowTime   int64
	windowSize      int
	minRTTThreshold int64
	logger          limit.Logger
	registry        core.MetricRegistry

	sample         *measurements.ImmutableSampleWindow
	inFlight       *int64
	nextUpdateTime int64
	mu             sync.RWMutex
}

// NewDefaultLimiterWithDefaults will create a DefaultLimit Limiter with the provided minimum config.
func NewDefaultLimiterWithDefaults(
	strategy core.Strategy,
	logger limit.Logger,
	registry core.MetricRegistry,
) (*DefaultLimiter, error) {
	return NewDefaultLimiter(
		limit.NewDefaultVegasLimit(logger, registry),
		defaultMinWindowTime,
		defaultMaxWindowTime,
		defaultMinRTTThreshold,
		defaultWindowSize,
		strategy,
		logger,
		registry,
	)
}

// NewDefaultLimiter creates a new DefaultLimiter.
func NewDefaultLimiter(
	limit core.Limit,
	minWindowTime int64,
	maxWindowTime int64,
	minRTTThreshold int64,
	windowSize int,
	strategy core.Strategy,
	logger limit.Logger,
	registry core.MetricRegistry,
) (*DefaultLimiter, error) {
	if limit == nil {
		return nil, fmt.Errorf("limit must be provided")
	}
	if strategy == nil {
		return nil, fmt.Errorf("stratewy must be provided")
	}
	if minWindowTime < defaultMinWindowTime {
		return nil, fmt.Errorf("minWindowTime must be >= %d", defaultMinWindowTime)
	}
	if maxWindowTime < defaultMaxWindowTime {
		return nil, fmt.Errorf("maxWindowTime must be >= %d", defaultMaxWindowTime)
	}
	if maxWindowTime < minWindowTime {
		return nil, fmt.Errorf("minWindowTime must be <= maxWindowTime")
	}
	if windowSize < 10 {
		return nil, fmt.Errorf("windowSize must be >= 10")
	}

	inFlight := int64(0)

	strategy.SetLimit(limit.EstimatedLimit())
	return &DefaultLimiter{
		limit:           limit,
		strategy:        strategy,
		minWindowTime:   minWindowTime,
		maxWindowTime:   maxWindowTime,
		minRTTThreshold: minRTTThreshold,
		windowSize:      windowSize,
		inFlight:        &inFlight,
		logger:          logger,
		registry:        registry,
	}, nil
}

func (l *DefaultLimiter) Acquire(ctx context.Context) (core.Listener, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Did we exceed the limit?
	token, ok := l.strategy.TryAcquire(ctx)
	if !ok || token == nil {
		return nil, false
	}

	startTime := time.Now().UnixNano()
	currentMaxInFlight := atomic.AddInt64(l.inFlight, 1)
	return &DefaultListener{
		currentMaxInFlight: currentMaxInFlight,
		inFlight:           l.inFlight,
		token:              token,
		startTime:          startTime,
		minRTTThreshold:    l.minRTTThreshold,
		limiter:            l,
		nextUpdateTime:     l.nextUpdateTime,
	}, true

}

func (l *DefaultLimiter) updateAndGetSample(
	f func(sample measurements.ImmutableSampleWindow) measurements.ImmutableSampleWindow,
) (measurements.ImmutableSampleWindow, measurements.ImmutableSampleWindow) {
	l.mu.Lock()
	defer l.mu.Unlock()
	before := *l.sample
	after := f(before)
	l.sample = &after
	return before, after
}

func (l *DefaultLimiter) isWindowReady(sample measurements.ImmutableSampleWindow) bool {
	return sample.CandidateRTTNanoseconds() < math.MaxInt64 && sample.SampleCount() > l.windowSize
}

func (l *DefaultLimiter) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.limit.EstimatedLimit()
}

func (l DefaultLimiter) String() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	rttCandidate := int64(-1)
	if l.sample != nil {
		rttCandidate = l.sample.CandidateRTTNanoseconds() / 1000
	}
	return fmt.Sprintf(
		"DefaultLimiter{RTTCandidate=%d ms, maxInFlight=%d, limit=%d, strategy=%v}",
		rttCandidate, l.inFlight, l.limit, l.strategy)
}
