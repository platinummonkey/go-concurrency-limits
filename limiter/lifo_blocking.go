package limiter

import (
	"context"
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/core"
	"sync"
	"time"
)

type lifoElement struct {
	id          uint64
	ctx         context.Context
	releaseChan chan core.Listener
	next, prev  *lifoElement
}

func (e *lifoElement) setListener(listener core.Listener) {
	select {
	case e.releaseChan <- listener:
		// noop
	default:
		// timeout has expired
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

func (q *lifoQueue) push(ctx context.Context) (uint64, chan core.Listener) {
	q.mu.Lock()
	defer q.mu.Unlock()
	releaseChan := make(chan core.Listener, 1)
	if q.top != nil {
		id := q.top.id + 1
		if id == 0 { // on overflow, roll back to 1
			id = 1
		}
		q.top = &lifoElement{id: id, next: q.top, ctx: ctx, releaseChan: releaseChan}
		q.top.next.prev = q.top
		q.size++
		return id, releaseChan
	}
	q.size++
	q.top = &lifoElement{id: 1, ctx: ctx, releaseChan: releaseChan}
	return 1, releaseChan
}

func (q *lifoQueue) pop() *lifoElement {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size > 0 {
		prev := lifoElement(*q.top)
		next := q.top.next
		next.prev = nil
		q.top.next = nil
		q.top.prev = nil
		q.top = next
		q.size--
		return &prev
	}
	return nil
}

func (q *lifoQueue) peek() (uint64, context.Context) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.size > 0 {
		return q.top.id, q.top.ctx
	}
	return 0, nil
}

func (q *lifoQueue) remove(id uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 || q.size < id {
		return
	}
	// remove the item, worst case O(n)
	var prev *lifoElement
	cur := q.top
	for {
		if cur.id == id {
			next := cur.next
			if prev == nil {
				// at the top, just re-assign
				next.prev = nil
				q.top.next = nil
				q.top.prev = nil
				q.top = next
				q.size--
				return
			}
			next.prev = prev
			prev.next = cur.next
			q.size--
			return
		}
	}
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
		_, nextEventCtx := l.limiter.backlog.peek()
		listener, ok := l.limiter.delegate.Acquire(nextEventCtx)
		if ok && listener != nil {
			nextEvent := l.limiter.backlog.pop()
			nextEvent.setListener(listener)
		}
		// otherwise: still can't acquire the limit.  unblock will be called again next time the limit is released.
	}
}

func (l *LifoBlockingListener) OnDropped() {
	l.delegateListener.OnDropped()
	l.unblock()
}

func (l *LifoBlockingListener) OnIgnore() {
	l.delegateListener.OnIgnore()
	l.unblock()
}

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
	eventID, eventReleaseChan := l.backlog.push(ctx)
	select {
	case listener = <-eventReleaseChan:
		return listener
	case <-time.After(l.maxBacklogTimeout):
		// Remove the holder from the backlog.  This item is likely to be at the end of the
		// list so do a remove to minimize the number of items to traverse
		l.backlog.remove(eventID)
		return nil
	}
}

func (l *LifoBlockingLimiter) Acquire(ctx context.Context) (core.Listener, bool) {
	panic("implement me")
}

func (l *LifoBlockingLimiter) String() string {
	return fmt.Sprintf("LifoBlockingLimiter{delegate=%v, maxBacklogSize=%d, maxBacklogTimeout=%v}",
		l.delegate, l.maxBacklogSize, l.maxBacklogTimeout)
}
