package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
)

// blockUntilSignaled will wait for context cancellation, an unblock signal or timeout
// This method will return true if we were successfully signalled.
func blockUntilSignaled(ctx context.Context, c *sync.Cond, timeout time.Duration) bool {
	ready := make(chan struct{})

	go func() {
		c.L.Lock()
		defer c.L.Unlock()
		close(ready)
	}()

	if timeout > 0 {
		// use NewTimer over time.After so that we don't have to
		// wait for the timeout to elapse in order to release memory
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return false
		case <-ready:
			return true
		case <-timer.C:
			return false
		}
	}

	select {
	case <-ctx.Done():
		return false
	case <-ready:
		return true
	}
}

// BlockingLimiter implements a Limiter that blocks the caller when the limit has been reached.  The caller is
// blocked until the limiter has been released.  This limiter is commonly used in batch clients that use the limiter
// as a back-pressure mechanism.
type BlockingLimiter struct {
	logger   limit.Logger
	delegate core.Limiter
	c        *sync.Cond
	timeout  time.Duration
}

// NewBlockingLimiter will create a new blocking limiter
func NewBlockingLimiter(
	delegate core.Limiter,
	timeout time.Duration,
	logger limit.Logger,
) *BlockingLimiter {
	mu := sync.Mutex{}
	if timeout < 0 {
		timeout = 0
	}
	if logger == nil {
		logger = limit.NoopLimitLogger{}
	}
	return &BlockingLimiter{
		logger:   logger,
		delegate: delegate,
		c:        sync.NewCond(&mu),
		timeout:  timeout,
	}
}

// tryAcquire will block when attempting to acquire a token
func (l *BlockingLimiter) tryAcquire(ctx context.Context) (core.Listener, bool) {

	for {
		// if the context has already been cancelled, fail quickly
		if err := ctx.Err(); err != nil {
			l.logger.Debugf("context cancelled ctx=%v", ctx)
			return nil, false
		}

		// try to acquire a new token and return immediately if successful
		listener, ok := l.delegate.Acquire(ctx)
		if ok && listener != nil {
			l.logger.Debugf("delegate returned a listener ctx=%v", ctx)
			return listener, true
		}

		// We have reached the limit so block until:
		// - A token is released
		// - A timeout
		// - The context is cancelled
		l.logger.Debugf("Blocking waiting for release or timeout ctx=%v", ctx)
		if shouldAcquire := blockUntilSignaled(ctx, l.c, l.timeout); shouldAcquire {
			listener, ok := l.delegate.Acquire(ctx)
			if ok && listener != nil {
				l.logger.Debugf("delegate returned a listener ctx=%v", ctx)
				return listener, true
			}
		}
		l.logger.Debugf("blocking released, trying again to acquire ctx=%v", ctx)
	}
}

// Acquire a token from the limiter.  Returns `nil, false` if the limit has been exceeded.
// If acquired the caller must call one of the Listener methods when the operation has been completed to release
// the count.
//
// context Context for the request. The context is used by advanced strategies such as LookupPartitionStrategy.
func (l *BlockingLimiter) Acquire(ctx context.Context) (core.Listener, bool) {
	delegateListener, ok := l.tryAcquire(ctx)
	if !ok && delegateListener == nil {
		l.logger.Debugf("did not acquire ctx=%v", ctx)
		return nil, false
	}
	l.logger.Debugf("acquired, returning listener ctx=%v", ctx)
	return &DelegateListener{
		delegateListener: delegateListener,
		c:                l.c,
	}, true
}

func (l BlockingLimiter) String() string {
	return fmt.Sprintf("BlockingLimiter{delegate=%v}", l.delegate)
}
