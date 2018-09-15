package limit

import (
	"fmt"
	"math"
	"sync"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// AIMDLimit implements a Loss based dynamic Limit that does an additive increment as long as there are no errors and a
// multiplicative decrement when there is an error.
type AIMDLimit struct {
	limit        int
	backOffRatio float64

	mu sync.RWMutex
}

func NewDefaultAIMLimit() *AIMDLimit {
	return NewAIMDLimit(10, 0.9)
}

func NewAIMDLimit(
	initialLimit int,
	backOffRatio float64,
) *AIMDLimit {
	return &AIMDLimit{
		limit:        initialLimit,
		backOffRatio: backOffRatio,
	}
}

func (l *AIMDLimit) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.limit
}

func (l *AIMDLimit) Update(sample core.SampleWindow) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if sample.DidDrop() {
		l.limit = int(math.Max(1, math.Min(float64(l.limit-1), float64(int(float64(l.limit)*l.backOffRatio)))))
	} else if sample.MaxInFlight() >= l.limit {
		l.limit++
	}
	return
}

func (l *AIMDLimit) BackOffRatio() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.backOffRatio
}

func (l *AIMDLimit) String() string {
	return fmt.Sprintf("AIMDLimit{limit=%d, backOffRatio=%0.4f}", l.EstimatedLimit(), l.BackOffRatio())
}
