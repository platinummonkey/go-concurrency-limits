package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
)

// DeadlineLimiter that blocks the caller when the limit has been reached.  The caller is
// blocked until the limiter has been released, or a deadline has been passed.
type DeadlineLimiter struct {
	logger   limit.Logger
	delegate core.Limiter
	deadline time.Time

	mu     sync.Mutex
	notify chan struct{} // closed (and replaced) whenever a token is released
}

// NewDeadlineLimiter will create a new DeadlineLimiter that will wrap a limiter such that acquire will block until a
// provided deadline if the limit was reached instead of returning an empty listener immediately.
func NewDeadlineLimiter(
	delegate core.Limiter,
	deadline time.Time,
	logger limit.Logger,
) *DeadlineLimiter {
	if logger == nil {
		logger = limit.NoopLimitLogger{}
	}
	return &DeadlineLimiter{
		logger:   logger,
		delegate: delegate,
		deadline: deadline,
		notify:   make(chan struct{}),
	}
}

// currentNotify returns the current notification channel under the lock.
func (l *DeadlineLimiter) currentNotify() <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.notify
}

// broadcastRelease closes the current notify channel (waking all waiters) and
// installs a fresh one for the next round of waiters.
func (l *DeadlineLimiter) broadcastRelease() {
	l.mu.Lock()
	old := l.notify
	l.notify = make(chan struct{})
	l.mu.Unlock()
	close(old)
}

// tryAcquire will block when attempting to acquire a token
func (l *DeadlineLimiter) tryAcquire(ctx context.Context) (listener core.Listener, ok bool) {
	for {
		// if the context has already been cancelled, fail quickly
		if err := ctx.Err(); err != nil {
			l.logger.Debugf("context cancelled ctx=%v", ctx)
			return nil, false
		}

		// if the deadline has passed, fail quickly
		remaining := time.Until(l.deadline)
		if remaining <= 0 {
			return nil, false
		}

		// Capture the notify channel BEFORE trying to acquire so a release
		// that fires between the failed attempt and the select is not missed.
		notify := l.currentNotify()

		// try to acquire a new token and return immediately if successful
		listener, ok := l.delegate.Acquire(ctx)
		if ok && listener != nil {
			l.logger.Debugf("delegate returned a listener ctx=%v", ctx)
			return listener, true
		}

		// We have reached the limit so block until:
		// - A token is released (notify channel is closed)
		// - The deadline passes
		// - The context is cancelled
		l.logger.Debugf("Blocking waiting for release or timeout ctx=%v", ctx)
		remaining = time.Until(l.deadline)
		if remaining <= 0 {
			return nil, false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false
		case <-notify:
			timer.Stop()
			// token was released, loop and retry
		case <-timer.C:
			return nil, false
		}
		l.logger.Debugf("blocking released, trying again to acquire ctx=%v", ctx)
	}
}

// Acquire a token from the limiter.  Returns `nil, false` if the limit has been exceeded.
// If acquired the caller must call one of the Listener methods when the operation has been completed to release
// the count.
//
// context Context for the request. The context is used by advanced strategies such as LookupPartitionStrategy.
func (l *DeadlineLimiter) Acquire(ctx context.Context) (listener core.Listener, ok bool) {
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

// String implements Stringer for easy debugging.
func (l DeadlineLimiter) String() string {
	return fmt.Sprintf("DeadlineLimiter{delegate=%v}", l.delegate)
}
