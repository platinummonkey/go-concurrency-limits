package pool

import (
	"context"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

// FixedPool implements a fixed size blocking pool.
type FixedPool struct {
	limit    int
	ordering Ordering
	limiter  core.Limiter
}

// NewFixedPool creates a named fixed pool resource. You can use this to guard another resource from too many concurrent
// requests.
//
// use < 0 values for defaults, but fixedLimit and name are required.
func NewFixedPool(
	name string,
	ordering Ordering,
	fixedLimit int,
	windowSize int,
	minWindowTime time.Duration,
	maxWindowTime time.Duration,
	minRTTThreshold time.Duration,
	maxBacklog int,
	timeout time.Duration,
	logger limit.Logger,
	metricRegistry core.MetricRegistry,
) (*FixedPool, error) {
	if minWindowTime < 0 {
		minWindowTime = time.Millisecond * 250
	}
	if maxWindowTime < 0 {
		maxWindowTime = time.Millisecond * 500
	}
	if minRTTThreshold < 0 {
		minRTTThreshold = time.Millisecond * 10
	}
	if windowSize <= 0 {
		windowSize = 100
	}
	if timeout < 0 {
		timeout = 0
	}
	if logger == nil {
		logger = limit.NoopLimitLogger{}
	}
	if metricRegistry == nil {
		metricRegistry = core.EmptyMetricRegistryInstance
	}

	limitStrategy := strategy.NewPreciseStrategy(fixedLimit)
	defaultLimiter, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit(
			name,
			fixedLimit,
			metricRegistry,
		),
		minWindowTime.Nanoseconds(),
		maxWindowTime.Nanoseconds(),
		minRTTThreshold.Nanoseconds(),
		windowSize,
		limitStrategy,
		logger,
		metricRegistry,
	)
	if err != nil {
		return nil, err
	}

	var fp FixedPool
	switch ordering {
	case OrderingFIFO:
		fp = FixedPool{
			limit:    fixedLimit,
			limiter:  limiter.NewFifoBlockingLimiter(defaultLimiter, maxBacklog, timeout),
			ordering: ordering,
		}
	case OrderingLIFO:
		fp = FixedPool{
			limit:    fixedLimit,
			limiter:  limiter.NewLifoBlockingLimiter(defaultLimiter, maxBacklog, timeout),
			ordering: ordering,
		}
	default:
		fp = FixedPool{
			limit:    fixedLimit,
			limiter:  limiter.NewBlockingLimiter(defaultLimiter, timeout, logger),
			ordering: ordering,
		}
	}
	return &fp, nil
}

// Limit will return the configured limit
func (p *FixedPool) Limit() int {
	return p.limit
}

func (p *FixedPool) Ordering() Ordering {
	return p.ordering
}

// Acquire a token for the protected resource. This method will block until acquisition or the configured timeout
// has expired.
func (p *FixedPool) Acquire(ctx context.Context) (core.Listener, bool) {
	return p.limiter.Acquire(ctx)
}
