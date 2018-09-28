package limit

import (
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/core"
	"math"
	"sync"
)

// AIMDLimit implements a Loss based dynamic Limit that does an additive increment as long as there are no errors and a
// multiplicative decrement when there is an error.
type AIMDLimit struct {
	limit        int
	backOffRatio float64

	listeners []core.LimitChangeListener

	mu sync.RWMutex
}

// NewDefaultAIMLimit will create a default AIMDLimit.
func NewDefaultAIMLimit() *AIMDLimit {
	return NewAIMDLimit(10, 0.9)
}

// NewAIMDLimit will create a new AIMDLimit.
func NewAIMDLimit(
	initialLimit int,
	backOffRatio float64,
) *AIMDLimit {
	return &AIMDLimit{
		limit:        initialLimit,
		backOffRatio: backOffRatio,
		listeners:    make([]core.LimitChangeListener, 0),
	}
}

// EstimatedLimit returns the current estimated limit.
func (l *AIMDLimit) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.limit
}

// NotifyOnChange will register a callback to receive notification whenever the limit is updated to a new value.
func (l *AIMDLimit) NotifyOnChange(consumer core.LimitChangeListener) {
	l.mu.Lock()
	l.listeners = append(l.listeners, consumer)
	l.mu.Unlock()
}

// notifyListeners will call the callbacks on limit changes
func (l *AIMDLimit) notifyListeners(newLimit int) {
	for _, listener := range l.listeners {
		listener(newLimit)
	}
}

// OnSample the concurrency limit using a new rtt sample.
func (l *AIMDLimit) OnSample(startTime int64, rtt int64, inFlight int, didDrop bool) {
	l.mu.Lock()
	l.mu.Unlock()

	if didDrop {
		l.limit = int(math.Max(1, math.Min(float64(l.limit-1), float64(int(float64(l.limit)*l.backOffRatio)))))
		l.notifyListeners(l.limit)
	} else if inFlight >= l.limit {
		l.limit++
		l.notifyListeners(l.limit)
	}
	return
}

// BackOffRatio return the current back-off-ratio for the AIMDLimit
func (l *AIMDLimit) BackOffRatio() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.backOffRatio
}

func (l *AIMDLimit) String() string {
	return fmt.Sprintf("AIMDLimit{limit=%d, backOffRatio=%0.4f}", l.EstimatedLimit(), l.BackOffRatio())
}
