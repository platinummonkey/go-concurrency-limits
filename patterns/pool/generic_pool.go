package pool

import (
	"context"
	"fmt"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
)

// Ordering define the pattern for ordering requests on Pool
type Ordering int

// The available options
const (
	OrderingRandom Ordering = iota
	OrderingFIFO
	OrderingLIFO
)

// Pool implements a generic blocking pool pattern.
type Pool struct {
	limiter core.Limiter
}

// NewPool creates a pool resource. You can use this to guard another resource from too many concurrent requests.
//
// use < 0 values for defaults, but delegateLimiter and name are required.
func NewPool(
	delegateLimiter core.Limiter,
	ordering Ordering,
	maxBacklog int,
	timeout time.Duration,
	logger limit.Logger,
	metricRegistry core.MetricRegistry,
) (*Pool, error) {
	if delegateLimiter == nil {
		return nil, fmt.Errorf("must specify a delegateLimiter")
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

	var p Pool
	switch ordering {
	case OrderingFIFO:
		p = Pool{
			limiter: limiter.NewFifoBlockingLimiter(delegateLimiter, maxBacklog, timeout),
		}
	case OrderingLIFO:
		p = Pool{
			limiter: limiter.NewLifoBlockingLimiter(delegateLimiter, maxBacklog, timeout),
		}
	default:
		p = Pool{
			limiter: limiter.NewBlockingLimiter(delegateLimiter, timeout, logger),
		}
	}

	return &p, nil
}

// Acquire a token for the protected resource. This method will block until acquisition or the configured timeout
// has expired.
func (p *Pool) Acquire(ctx context.Context) (core.Listener, bool) {
	return p.limiter.Acquire(ctx)
}
