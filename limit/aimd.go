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
	name         string
	limit        int
	backOffRatio float64

	listeners        []core.LimitChangeListener
	registry         core.MetricRegistry
	dropCounter      core.MetricSampleListener
	rttListener      core.MetricSampleListener
	inFlightListener core.MetricSampleListener

	mu sync.RWMutex
}

// NewDefaultAIMLimit will create a default AIMDLimit.
func NewDefaultAIMLimit(
	name string,
	registry core.MetricRegistry,
) *AIMDLimit {
	return NewAIMDLimit(name, 10, 0.9, registry)
}

// NewAIMDLimit will create a new AIMDLimit.
func NewAIMDLimit(
	name string,
	initialLimit int,
	backOffRatio float64,
	registry core.MetricRegistry,
) *AIMDLimit {
	if registry == nil {
		registry = core.EmptyMetricRegistryInstance
	}

	l := &AIMDLimit{
		name:             name,
		limit:            initialLimit,
		backOffRatio:     backOffRatio,
		listeners:        make([]core.LimitChangeListener, 0),
		registry:         registry,
		dropCounter:      registry.RegisterCount(core.PrefixMetricWithName(core.MetricDropped, name)),
		rttListener:      registry.RegisterTiming(core.PrefixMetricWithName(core.MetricMinRTT, name)),
		inFlightListener: registry.RegisterDistribution(core.PrefixMetricWithName(core.MetricInFlight, name)),
	}

	registry.RegisterGauge(
		core.PrefixMetricWithName(core.MetricLimit, name),
		core.NewIntMetricSupplierWrapper(l.EstimatedLimit),
	)
	return l
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

	l.rttListener.AddSample(float64(rtt))
	l.inFlightListener.AddSample(float64(inFlight))

	if didDrop {
		l.dropCounter.AddSample(1)
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
