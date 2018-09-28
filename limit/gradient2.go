package limit

import (
	"fmt"
	"math"
	"sync"
	
	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

// Gradient2Limit implements a double exponential smoothing gradient.
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
	minLimit                int
	queueSizeFunc           func(limit int) int
	smoothing               float64
	longRTTSampleListener   core.MetricSampleListener
	shortRTTSampleListener  core.MetricSampleListener
	queueSizeSampleListener core.MetricSampleListener
	maxDriftIntervals       int
	intervalsAbove          int

	mu        sync.RWMutex
	listeners []core.LimitChangeListener
	logger    Logger
	registry  core.MetricRegistry
}

// NewDefaultGradient2Limit create a default Gradient2Limit
func NewDefaultGradient2Limit(
	logger Logger,
	registry core.MetricRegistry,
) *Gradient2Limit {
	l, _ := NewGradient2Limit(
		4,
		1000,
		4,
		func(limit int) int { return 4 },
		0.2,
		5,
		10,
		100,
		logger,
		registry,
	)
	return l
}

// NewGradient2Limit will create a new Gradient2Limit
// @param initialLimit: Initial limit used by the limiter.
// @param maxConcurrency: Maximum allowable concurrency.  Any estimated concurrency will be capped.
// @param minLimit: Minimum concurrency limit allowed.  The minimum helps prevent the algorithm from adjust the limit
//                  too far down.  Note that this limit is not desirable when use as backpressure for batch apps.
// @param queueSizeFunc: Function to dynamically determine the amount the estimated limit can grow while
//                       latencies remain low as a function of the current limit.
// @param smoothing: Smoothing factor to limit how aggressively the estimated limit can shrink when queuing has been
//                   detected.  Value of 0.0 to 1.0 where 1.0 means the limit is completely replicated by the new estimate.
// @param driftMultiplier: Maximum multiple of the fast window after which we need to reset the limiter.
// @param shortWindow: short time window for the exponential avg recordings.
// @param longWindow: long time window for the exponential avg recordings.
// @param registry: metric registry to publish metrics
func NewGradient2Limit(
	initialLimit int, // Initial limit used by the limiter
	maxConurrency int,
	minLimit int,
	queueSizeFunc func(limit int) int,
	smoothing float64,
	driftMultiplier float64,
	shortWindow int,
	longWindow int,
	logger Logger,
	registry core.MetricRegistry,
) (*Gradient2Limit, error) {
	if smoothing > 1.0 || smoothing < 0 {
		smoothing = 0.2
	}
	if maxConurrency <= 0 {
		maxConurrency = 1000
	}
	if minLimit <= 0 {
		minLimit = 4
	}
	if driftMultiplier <= 0 {
		driftMultiplier = 5
	}
	if shortWindow < 0 {
		shortWindow = 10
	}
	if longWindow < 0 {
		longWindow = 100
	}
	if logger == nil {
		logger = NoopLimitLogger{}
	}
	if registry == nil {
		registry = core.EmptyMetricRegistryInstance
	}

	if minLimit > maxConurrency {
		return nil, fmt.Errorf("minLimit must be <= maxConcurrency")
	}
	if queueSizeFunc == nil {
		// set the default
		queueSizeFunc = func(limit int) int { return 4 }
	}
	if initialLimit <= 0 {
		initialLimit = 4
	}

	return &Gradient2Limit{
		estimatedLimit:          float64(initialLimit),
		maxLimit:                maxConurrency,
		minLimit:                minLimit,
		queueSizeFunc:           queueSizeFunc,
		smoothing:               smoothing,
		shortRTT:                measurements.NewExponentialAverageMeasurement(shortWindow, 10),
		longRTT:                 measurements.NewExponentialAverageMeasurement(longWindow, 10),
		maxDriftIntervals:       int(float64(shortWindow) * driftMultiplier),
		longRTTSampleListener:   registry.RegisterDistribution(core.MetricMinRTT),
		shortRTTSampleListener:  registry.RegisterDistribution(core.MetricWindowMinRTT),
		queueSizeSampleListener: registry.RegisterDistribution(core.MetricWindowQueueSize),
		listeners:               make([]core.LimitChangeListener, 0),
		logger:                  logger,
		registry:                registry,
	}, nil
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
	gradient := math.Max(0.5, math.Min(1.0, longRTT/shortRTT))

	// Don't grow the limit if we are app limited
	if float64(inFlight) < l.estimatedLimit/2 {
		return
	}

	newLimit := l.estimatedLimit*gradient + float64(queueSize)
	if newLimit < l.estimatedLimit {
		newLimit = math.Max(float64(l.minLimit), l.estimatedLimit+(1-l.smoothing)+l.smoothing*newLimit)
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
