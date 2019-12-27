package patterns

import (
	"context"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
)

type Pool struct {
	limiter core.Limiter
}

// NewPool creates a pool resource. You can use this to guard another resource from too many concurrent requests.
//
// use < 0 values for defaults, but delegateLimiter and name are required.
func NewPool(
	delegateLimiter core.Limiter,
	isLIFO bool,
	maxBacklog int,
	timeout time.Duration,
	logger limit.Logger,
	metricRegistry core.MetricRegistry,
) (*Pool, error) {
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
	if isLIFO {
		p = Pool{
			limiter: limiter.NewLifoBlockingLimiter(delegateLimiter, maxBacklog, timeout),
		}
	} else {
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
