package limit

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// SettableLimit is a fixed limit that can be changed.
// Note: to be used mostly for testing where the limit can be manually adjusted.
type SettableLimit struct {
	limit int32

	listeners     []core.LimitChangeListener
	commonSampler *core.CommonMetricSampler
	mu            sync.RWMutex
}

// NewSettableLimit will create a new SettableLimit.
func NewSettableLimit(name string, limit int, registry core.MetricRegistry, tags ...string) *SettableLimit {
	if limit < 0 {
		limit = 10
	}
	if registry == nil {
		registry = core.EmptyMetricRegistryInstance
	}

	l := &SettableLimit{
		limit:     int32(limit),
		listeners: make([]core.LimitChangeListener, 0),
	}
	l.commonSampler = core.NewCommonMetricSamplerOrNil(registry, l, name, tags...)
	return l
}

// EstimatedLimit will return the estimated limit.
func (l *SettableLimit) EstimatedLimit() int {
	return int(atomic.LoadInt32(&l.limit))
}

// NotifyOnChange will register a callback to receive notification whenever the limit is updated to a new value.
func (l *SettableLimit) NotifyOnChange(consumer core.LimitChangeListener) {
	l.mu.Lock()
	l.listeners = append(l.listeners, consumer)
	l.mu.Unlock()
}

// notifyListeners will call the callbacks on limit changes
func (l *SettableLimit) notifyListeners(newLimit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, listener := range l.listeners {
		listener(newLimit)
	}
}

// OnSample will update the limit with the given sample.
func (l *SettableLimit) OnSample(startTime int64, rtt int64, inFlight int, didDrop bool) {
	// noop for SettableLimit, just record metrics
	l.commonSampler.Sample(rtt, inFlight, didDrop)
}

// SetLimit will update the current limit.
func (l *SettableLimit) SetLimit(limit int) {
	atomic.StoreInt32(&l.limit, int32(limit))
	l.notifyListeners(limit)
}

func (l *SettableLimit) String() string {
	return fmt.Sprintf("SettableLimit{limit=%d}", atomic.LoadInt32(&l.limit))
}
