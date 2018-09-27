package limit

import (
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/core"
	"math"
	"sync"
)

type Gradient2Limit struct {
	// Estimated concurrency limit based on our algorithm
	estimatedLimit float64
	// Tracks a measurement of the short time, and more volatile, RTT meant to represent the current system latency
	shortRTT core.MeasurementInterface
	// Tracks a measurement of the long term, less volatile, RTT meant to represent the baseline latency.  When the system
	// is under load this number is expect to trend higher.
	longRTT core.MeasurementInterface
	// Maximum allowed limit providing an upper bound failsafe
	maxLimit int
	// Minimum allowed limit providing a lower bound failsafe
	minLimit int
	queueSizeFunc              func(estimatedLimit int) int
	smoothing                  float64
	longRTTSampleListener      core.MetricSampleListener
	shortRTTSampleListener     core.MetricSampleListener
	queueSizeSampleListener    core.MetricSampleListener
	maxDriftIntervals int
	intervalsAbove int

	mu sync.RWMutex
	listeners []core.LimitChangeListener
	logger Logger
}

// EstimatedLimit returns the current estimated limit.
func (l *Gradient2Limit) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return int(l.estimatedLimit)
}

// NotifyOnChange will register a callback to receive notification whenever the limit is updated to a new value.
func (l *Gradient2Limit) NotifyOnChange(consumer core.LimitChangeListener) {
	l.mu.Lock()
	l.listeners = append(l.listeners, consumer)
	l.mu.Unlock()
}

// notifyListeners will call the callbacks on limit changes
func (l *Gradient2Limit) notifyListeners(newLimit int) {
	for _, listener := range l.listeners {
		listener(newLimit)
	}
}

// OnSample the concurrency limit using a new rtt sample.
func (l *Gradient2Limit) OnSample(startTime int64, rtt int64, inFlight int, didDrop bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	queueSize := l.queueSizeFunc(int(l.estimatedLimit))

	shortRTT, _ := l.shortRTT.Add(float64(rtt))
	longRTT, _ := l.longRTT.Add(float64(rtt))

	// Under steady state we expect the short and long term RTT to whipsaw.  We can identify that a system is under
	// long term load when there is no crossover detected for a certain number of internals, normally a multiple of
	// the short term RTT window.  Since both short and long term RTT trend higher this state results in the limit
	// slowly trending upwards, increasing the queue and latency.  To mitigate this we drop both the limit and
	// long term latency value to effectively probe for less queueing and better latency.
	if shortRTT > longRTT {
		l.intervalsAbove++
		if l.intervalsAbove > l.maxDriftIntervals {
			l.intervalsAbove = 0
			newLimit := l.minLimit
			if queueSize > l.minLimit {
				newLimit = queueSize
			}
			l.longRTT.Reset()
			l.estimatedLimit = float64(newLimit)
			l.notifyListeners(newLimit)
			return
		}
	} else {
		l.intervalsAbove = 0
	}

	l.shortRTTSampleListener.AddSample(shortRTT)
	l.longRTTSampleListener.AddSample(longRTT)
	l.queueSizeSampleListener.AddSample(float64(queueSize))

	// Rtt could be higher than rtt_noload because of smoothing rtt noload updates
	// so set to 1.0 to indicate no queuing.  Otherwise calculate the slope and don't
	// allow it to be reduced by more than half to avoid aggressive load-shedding due to
	// outliers.
	gradient := math.Max(0.5, math.Min(1.0, longRTT / shortRTT))

	// Don't grow the limit if we are app limited
	if float64(inFlight) < l.estimatedLimit / 2 {
		return
	}

	newLimit := l.estimatedLimit * gradient + float64(queueSize)
	if newLimit < l.estimatedLimit {
		newLimit = math.Max(float64(l.minLimit), l.estimatedLimit + (1 - l.smoothing) + l.smoothing * newLimit)
	}
	newLimit = math.Max(float64(queueSize), math.Min(float64(l.maxLimit), newLimit))

	if newLimit != l.estimatedLimit && l.logger.IsDebugEnabled() {
		l.logger.Debugf("new limit=%0.2f, shortRTT=%d ms, longRTT=%d ms, queueSize=%d, gradient=%0.2f",
			newLimit, shortRTT/1e6, longRTT/1e6, queueSize, gradient)
	}

	l.estimatedLimit = newLimit
	l.notifyListeners(int(l.estimatedLimit))
}

func (l *Gradient2Limit) String() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return fmt.Sprintf("Gradient2Limit{limit=%d}", int(l.estimatedLimit))
}
