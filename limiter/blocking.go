package limiter

import (
	"context"
	"fmt"
	"sync"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// BlockingListener wraps the wrapped Limiter's Listener to correctly handle releasing blocked connections
type BlockingListener struct {
	delegateListener core.Listener
	c                *sync.Cond
}

func (l *BlockingListener) unblock() {
	l.c.Broadcast()
}

// OnDropped is called to indicate the request failed and was dropped due to being rejected by an external limit or
// hitting a timeout.  Loss based Limit implementations will likely do an aggressive reducing in limit when this
// happens.
func (l *BlockingListener) OnDropped() {
	l.delegateListener.OnDropped()
	l.unblock()
}

// OnIgnore is called to indicate the operation failed before any meaningful RTT measurement could be made and
// should be ignored to not introduce an artificially low RTT.
func (l *BlockingListener) OnIgnore() {
	l.delegateListener.OnIgnore()
	l.unblock()
}

// OnSuccess is called as a notification that the operation succeeded and internally measured latency should be
// used as an RTT sample.
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

// NewBlockingLimiter will create a new blocking limiter
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

// Acquire a token from the limiter.  Returns an Optional.empty() if the limit has been exceeded.
// If acquired the caller must call one of the Listener methods when the operation has been completed to release
// the count.
//
// context Context for the request. The context is used by advanced strategies such as LookupPartitionStrategy.
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
