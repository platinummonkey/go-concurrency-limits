package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

type lifoElement struct {
	ctx         context.Context
	releaseChan chan core.Listener
	next, prev  *lifoElement
	evicted     bool
}

func (e *lifoElement) setListener(listener core.Listener) bool {
	select {
	case e.releaseChan <- listener:
		close(e.releaseChan)
		return true
	default:
		// timeout has expired
		return false
	}
}

func (q *lifoQueue) evictionFunc(e *lifoElement) func() {
	return func() {
		q.mu.Lock()
		defer q.mu.Unlock()

		// Prevent multiple invocations from
		// corrupting the queue state.
		if e.evicted {
			return
		}

		e.evicted = true

		if e.prev == nil {
			q.top = e.next
		} else {
			e.prev.next = e.next
		}
		if e.next != nil {
			e.next.prev = e.prev
		}
		q.size--
	}
}

type lifoQueue struct {
	top  *lifoElement
	size uint64
	mu   sync.RWMutex
}

func (q *lifoQueue) len() uint64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size
}

func (q *lifoQueue) push(ctx context.Context) (func(), chan core.Listener) {
	q.mu.Lock()
	defer q.mu.Unlock()
	releaseChan := make(chan core.Listener, 1)
	if q.top != nil {
		q.top = &lifoElement{next: q.top, ctx: ctx, releaseChan: releaseChan}
		q.top.next.prev = q.top
		q.size++
		return q.evictionFunc(q.top), releaseChan
	}
	q.size++
	q.top = &lifoElement{ctx: ctx, releaseChan: releaseChan}
	return q.evictionFunc(q.top), releaseChan
}

func (q *lifoQueue) pop() *lifoElement {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size > 0 {
		prev := *q.top
		next := q.top.next
		if next != nil {
			next.prev = nil
		}
		q.top.next = nil
		q.top.prev = nil
		q.top = next
		q.size--
		return &prev
	}
	return nil
}

func (q *lifoQueue) peek() (func(), *lifoElement) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.size > 0 {
		return q.evictionFunc(q.top), q.top
	}
	return nil, nil
}

// LifoBlockingListener implements a blocking listener for the LifoBlockingListener
type LifoBlockingListener struct {
	delegateListener core.Listener
	limiter          *LifoBlockingLimiter
}

func (l *LifoBlockingListener) unblock() {
	l.limiter.mu.Lock()
	defer l.limiter.mu.Unlock()

	if l.limiter.backlog.len() > 0 {

		evict, nextEvent := l.limiter.backlog.peek()
		listener, ok := l.limiter.delegate.Acquire(nextEvent.ctx)

		if ok && listener != nil {
			// We successfully acquired a listener from the
			// delegate. Now we can evict the element from
			// the queue
			evict()

			// If the listener is not accepted due to subtle timings
			// between setListener being invoked and the element
			// expiration elapsing we need to be sure to release it.
			if accepted := nextEvent.setListener(listener); !accepted {
				listener.OnIgnore()
			}
		}
		// otherwise: still can't acquire the limit.  unblock will be called again next time the limit is released.
	}
}

// OnDropped is called to indicate the request failed and was dropped due to being rejected by an external limit or
// hitting a timeout.  Loss based Limit implementations will likely do an aggressive reducing in limit when this
// happens.
func (l *LifoBlockingListener) OnDropped() {
	l.delegateListener.OnDropped()
	l.unblock()
}

// OnIgnore is called to indicate the operation failed before any meaningful RTT measurement could be made and
// should be ignored to not introduce an artificially low RTT.
func (l *LifoBlockingListener) OnIgnore() {
	l.delegateListener.OnIgnore()
	l.unblock()
}

// OnSuccess is called as a notification that the operation succeeded and internally measured latency should be
// used as an RTT sample.
func (l *LifoBlockingListener) OnSuccess() {
	l.delegateListener.OnSuccess()
	l.unblock()
}

// LifoBlockingLimiter implements a Limiter that blocks the caller when the limit has been reached.  This strategy
// ensures the resource is properly protected but favors availability over latency by not fast failing requests when
// the limit has been reached.  To help keep success latencies low and minimize timeouts any blocked requests are
// processed in last in/first out order.
//
// Use this limiter only when the concurrency model allows the limiter to be blocked.
type LifoBlockingLimiter struct {
	delegate          core.Limiter
	maxBacklogSize    uint64
	maxBacklogTimeout time.Duration

	backlog lifoQueue
	c       *sync.Cond
	mu      sync.RWMutex
}

// NewLifoBlockingLimiter will create a new LifoBlockingLimiter
func NewLifoBlockingLimiter(
	delegate core.Limiter,
	maxBacklogSize int,
	maxBacklogTimeout time.Duration,
) *LifoBlockingLimiter {
	if maxBacklogSize <= 0 {
		maxBacklogSize = 100
	}
	if maxBacklogTimeout == 0 {
		maxBacklogTimeout = time.Millisecond * 1000
	}
	mu := sync.Mutex{}
	return &LifoBlockingLimiter{
		delegate:          delegate,
		maxBacklogSize:    uint64(maxBacklogSize),
		maxBacklogTimeout: maxBacklogTimeout,
		backlog:           lifoQueue{},
		c:                 sync.NewCond(&mu),
	}
}

// NewLifoBlockingLimiterWithDefaults will create a new LifoBlockingLimiter with default values.
func NewLifoBlockingLimiterWithDefaults(
	delegate core.Limiter,
) *LifoBlockingLimiter {
	return NewLifoBlockingLimiter(delegate, 100, time.Millisecond*1000)
}

func (l *LifoBlockingLimiter) tryAcquire(ctx context.Context) core.Listener {
	// Try to acquire a token and return immediately if successful
	listener, ok := l.delegate.Acquire(ctx)
	if ok && listener != nil {
		return listener
	}

	// Restrict backlog size so the queue doesn't grow unbounded during an outage
	if l.backlog.len() >= l.maxBacklogSize {
		return nil
	}

	// Create a holder for a listener and block until a listener is released by another
	// operation.  Holders will be unblocked in LIFO order
	evict, eventReleaseChan := l.backlog.push(ctx)

	select {
	case listener = <-eventReleaseChan:
		// If we have received a listener then that means
		// that 'unblock' has already evicted this element
		// from the queue for us.
		return listener
	case <-time.After(l.maxBacklogTimeout):
		// Remove the holder from the backlog.
		evict()
		return nil
	}
}

// Acquire a token from the limiter.  Returns an Optional.empty() if the limit has been exceeded.
// If acquired the caller must call one of the Listener methods when the operation has been completed to release
// the count.
//
// context Context for the request. The context is used by advanced strategies such as LookupPartitionStrategy.
func (l *LifoBlockingLimiter) Acquire(ctx context.Context) (core.Listener, bool) {
	delegateListener := l.tryAcquire(ctx)
	if delegateListener == nil {
		return nil, false
	}
	return &LifoBlockingListener{
		delegateListener: delegateListener,
		limiter:          l,
	}, true
}

func (l *LifoBlockingLimiter) String() string {
	return fmt.Sprintf("LifoBlockingLimiter{delegate=%v, maxBacklogSize=%d, maxBacklogTimeout=%v}",
		l.delegate, l.maxBacklogSize, l.maxBacklogTimeout)
}
