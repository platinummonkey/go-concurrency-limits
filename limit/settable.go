package limit

import (
	"fmt"
	"sync/atomic"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// SettableLimit is a fixed limit that can be changed.
// Note: to be used mostly for testing where the limit can be manually adjusted.
type SettableLimit struct {
	limit *int32
}

// NewSettableLimit will create a new SettableLimit.
func NewSettableLimit(limit int) *SettableLimit {
	l := int32(limit)
	return &SettableLimit{
		limit: &l,
	}
}

// EstimatedLimit will return the estimated limit.
func (l *SettableLimit) EstimatedLimit() int {
	return int(atomic.LoadInt32(l.limit))
}

// OnSample will update the limit with the given sample.
func (l *SettableLimit) OnSample(sample core.SampleWindow) {
	// noop for SettableLimit
}

// SetLimit will update the current limit.
func (l *SettableLimit) SetLimit(limit int) {
	atomic.StoreInt32(l.limit, int32(limit))
}

func (l SettableLimit) String() string {
	return fmt.Sprintf("SettableLimit{limit=%d}", atomic.LoadInt32(l.limit))
}
