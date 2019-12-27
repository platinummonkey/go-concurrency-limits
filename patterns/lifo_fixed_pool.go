package patterns

import (
	"context"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type LIFOFixedPool struct {
	limit int
	limiter core.Limiter
}

// NewLIFOFixedPool creates a named LIFO fixed pool resource. You can use this to guard another resource from too many
// concurrent requests.
//
// use < 0 values for defaults, but fixedLimit and name are required.
func NewLIFOFixedPool(
	name string,
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
		minWindowTime = time.Millisecond*250
	}
	if maxWindowTime < 0 {
		maxWindowTime = time.Millisecond*500
	}
	if minRTTThreshold < 0 {
		minRTTThreshold = time.Millisecond*10
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

	limitStrategy := strategy.NewSimpleStrategy(fixedLimit)
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

	fp := &FixedPool{
		limit:   fixedLimit,
		limiter: limiter.NewLifoBlockingLimiter(defaultLimiter, maxBacklog, timeout),
	}
	return fp, nil
}

// Limit will return the configured limit
func (p *LIFOFixedPool) Limit() int {
	return p.limit
}

// Acquire a token for the protected resource. This method will block until acquisition or the configured timeout
// has expired.
func (p *LIFOFixedPool) Acquire(ctx context.Context) (core.Listener, bool) {
	return p.limiter.Acquire(ctx)
}
