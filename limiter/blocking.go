package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
)

// BlockingLimiter implements a Limiter that blocks the caller when the limit has been reached.  The caller is
// blocked until the limiter has been released.  This limiter is commonly used in batch clients that use the limiter
// as a back-pressure mechanism.
//
// Internally a notify-channel is used instead of sync.Cond to avoid the lost-wakeup race: the current channel is
// captured before each failed acquisition attempt, so a release that fires between the failed acquire and the
// blocking select will already have closed the channel, causing the select to proceed immediately.
type BlockingLimiter struct {
	logger   limit.Logger
	delegate core.Limiter
	timeout  time.Duration

	mu     sync.Mutex
	notify chan struct{} // closed (and replaced) whenever a token is released
}

// NewBlockingLimiter will create a new blocking limiter
func NewBlockingLimiter(
	delegate core.Limiter,
	timeout time.Duration,
	logger limit.Logger,
) *BlockingLimiter {
	if timeout < 0 {
		timeout = 0
	}
	if logger == nil {
		logger = limit.NoopLimitLogger{}
	}
	return &BlockingLimiter{
		logger:   logger,
		delegate: delegate,
		timeout:  timeout,
		notify:   make(chan struct{}),
	}
}

// currentNotify returns the current notification channel under the lock.
// Callers must capture this BEFORE the failed acquire attempt so that a
// release between the attempt and the blocking select is not missed.
func (l *BlockingLimiter) currentNotify() <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.notify
}

// broadcastRelease closes the current notify channel (waking all waiters) and
// installs a fresh one for the next round of waiters.
func (l *BlockingLimiter) broadcastRelease() {
	l.mu.Lock()
	old := l.notify
	l.notify = make(chan struct{})
	l.mu.Unlock()
	close(old)
}

// tryAcquire will block when attempting to acquire a token
func (l *BlockingLimiter) tryAcquire(ctx context.Context) (core.Listener, bool) {
	for {
		// if the context has already been cancelled, fail quickly
		if err := ctx.Err(); err != nil {
			l.logger.Debugf("context cancelled ctx=%v", ctx)
			return nil, false
		}

		// Capture the notify channel BEFORE trying to acquire.
		// If a release fires between here and the select below the channel
		// will already be closed, so the select proceeds without blocking.
		notify := l.currentNotify()

		// try to acquire a new token and return immediately if successful
		listener, ok := l.delegate.Acquire(ctx)
		if ok && listener != nil {
			l.logger.Debugf("delegate returned a listener ctx=%v", ctx)
			return listener, true
		}

		// We have reached the limit so block until:
		// - A token is released (notify channel is closed)
		// - A timeout
		// - The context is cancelled
		l.logger.Debugf("Blocking waiting for release or timeout ctx=%v", ctx)
		if l.timeout > 0 {
			timer := time.NewTimer(l.timeout)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, false
			case <-notify:
				timer.Stop()
				// token was released, loop and retry
			case <-timer.C:
				// per-attempt timeout: loop back and check context before retrying
			}
		} else {
			select {
			case <-ctx.Done():
				return nil, false
			case <-notify:
				// token was released, loop and retry
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
		onRelease:        l.broadcastRelease,
	}, true
}

func (l BlockingLimiter) String() string {
	return fmt.Sprintf("BlockingLimiter{delegate=%v}", l.delegate)
}
