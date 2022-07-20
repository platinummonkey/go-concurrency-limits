package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
)

const longBlockingTimeout = time.Hour * 24 * 30 * 12 * 100 // 100 years

// timeoutWaiter will wait for a timeout or unblock signal
type timeoutWaiter struct {
	timeoutSig chan struct{}
	closerSig  chan struct{}
	c          *sync.Cond
	once       sync.Once
	timeout    time.Duration
}

func newTimeoutWaiter(c *sync.Cond, timeout time.Duration) *timeoutWaiter {
	return &timeoutWaiter{
		timeoutSig: make(chan struct{}),
		closerSig:  make(chan struct{}),
		c:          c,
		timeout:    timeout,
	}
}

func (w *timeoutWaiter) start() {
	// start two routines, one runner to signal, another blocking to wait and call unblock
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		wg.Done()
		w.run()
	}()
	go func() {
		wg.Done()
		w.c.L.Lock()
		w.c.Wait()
		w.c.L.Unlock()
		w.unblock()
	}()
	wg.Wait()
}

func (w *timeoutWaiter) run() {
	if w.timeout > 0 {
		select {
		case <-w.closerSig:
			close(w.timeoutSig)
			return
		case <-time.After(w.timeout):
			// call unblock
			close(w.timeoutSig)
			return
		}
	}
	select {
	case <-w.closerSig:
		close(w.timeoutSig)
		return
	}
}

func (w *timeoutWaiter) unblock() {
	w.once.Do(func() {
		close(w.closerSig)
	})
}

// wait blocks until we've timed out
func (w *timeoutWaiter) wait() <-chan struct{} {
	return w.timeoutSig
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
	if timeout <= 0 {
		timeout = longBlockingTimeout
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
		l.c.L.Lock()
		// if the context has already been cancelled, fail quickly
		if err := ctx.Err(); err != nil {
			l.logger.Debugf("context cancelled ctx=%v", ctx)
			l.c.L.Unlock()
			return nil, false
		}

		// try to acquire a new token and return immediately if successful
		listener, ok := l.delegate.Acquire(ctx)
		if ok && listener != nil {
			l.logger.Debugf("delegate returned a listener ctx=%v", ctx)
			l.c.L.Unlock()
			return listener, true
		}
		l.c.L.Unlock()

		// We have reached the limit so block until:
		// - A token is released
		// - A timeout
		// - The context is cancelled
		timeoutWaiter := newTimeoutWaiter(l.c, l.timeout)
		timeoutWaiter.start()
		select {
		case <-timeoutWaiter.wait():
			l.logger.Debugf("blocking released, trying again to acquire ctx=%v", ctx)
		case <-ctx.Done():
			l.logger.Debugf("blocking released, context's has been cancelled ctx=%v", ctx)
			return nil, false
		}
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
