package limiter

import (
	"context"
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/core"
	"sync"
)

// BlockingListener wraps the wrapped Limiter's Listener to correctly handle releasing blocked connections
type BlockingListener struct {
	delegateListener core.Listener
	c                *sync.Cond
}

func (l *BlockingListener) unblock() {
	l.c.Broadcast()
}

func (l *BlockingListener) OnDropped() {
	l.delegateListener.OnDropped()
	l.unblock()
}

func (l *BlockingListener) OnIgnore() {
	l.delegateListener.OnIgnore()
	l.unblock()
}

func (l *BlockingListener) OnSuccess() {
	l.delegateListener.OnSuccess()
	l.unblock()
}

// BlockingLimiter implements a Limiter that blocks the caller when the limit has been reached.  The caller is
// blocked until the limiter has been released.  This limiter is commonly used in batch clients that use the limiter
// as a back-pressure mechanism.
type BlockingLimiter struct {
	delegate core.Limiter
	c        *sync.Cond
}

func NewBlockingLimiter(
	delegate core.Limiter,
) *BlockingLimiter {
	mu := sync.Mutex{}
	return &BlockingLimiter{
		delegate: delegate,
		c:        sync.NewCond(&mu),
	}
}

// tryAcquire will block when attempting to acquire a token
func (l *BlockingLimiter) tryAcquire(ctx context.Context) core.Listener {
	l.c.L.Lock()
	defer l.c.L.Unlock()
	for {
		// try to acquire a new token and return immediately if successful
		listener, ok := l.delegate.Acquire(ctx)
		if ok && listener != nil {
			return listener
		}

		// We have reached the limit so block until a token is released
		l.c.Wait()
	}
}

func (l *BlockingLimiter) Acquire(ctx context.Context) (core.Listener, bool) {
	delegateListener := l.tryAcquire(ctx)
	if delegateListener == nil {
		return nil, false
	}
	return &BlockingListener{
		delegateListener: delegateListener,
		c:                l.c,
	}, true
}

func (l BlockingLimiter) String() string {
	return fmt.Sprintf("BlockingLimiter{delegate=%v}", l.delegate)
}
