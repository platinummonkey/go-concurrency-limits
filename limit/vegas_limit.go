package limit

import (
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/platinummonkey/go-concurrency-limits/limit/functions"
	"github.com/platinummonkey/go-concurrency-limits/core"
)

// VegasLimit implements a Limiter based on TCP Vegas where the limit increases by alpha if the queue_use is
// small < alpha and decreases by alpha if the queue_use is large > beta.
//
// Queue size is calculated using the formula,
//   queue_use = limit − BWE×RTTnoLoad = limit × (1 − RTTnoLoad/RTTactual)
//
// For traditional TCP Vegas alpha is typically 2-3 and beta is typically 4-6.  To allow for better growth and stability
// at higher limits we set alpha=Max(3, 10% of the current limit) and beta=Max(6, 20% of the current limit).
type VegasLimit struct {
	estimatedLimit float64
	maxLimit int
	rttNoLoad int64
	smoothing float64
	alphaFunc func(estimatedLimit int) int
	betaFunc func(estimatedLimit int) int
	thresholdFunc func(estimatedLimit int) int
	increaseFunc func(estimatedLimit float64) float64
	decreaseFunc func(estimatedLimit float64) float64
	rttSampleListener core.MetricSampleListener
	probeMultipler int
	probeCountdown int

	registry core.MetricRegistry
	logger Logger
	mu sync.RWMutex
}

// NewVegasLimit will create a new VegasLimit.
func NewVegasLimitWithRegistry(
	initialLimit int,
	maxConcurrency int,
	smoothing float64,
	alphaFunc func(estimatedLimit int) int,
	betaFunc func(estimatedLimit int) int,
	thresholdFunc func(estimatedLimit int) int,
	increaseFunc func(estimatedLimit float64) float64,
	decreaseFunc func(estimatedLimit float64) float64,
	probeMultiplier int,
	probeCountdown int,
	logger Logger,
	registry core.MetricRegistry,
) *VegasLimit {
	if initialLimit < 1 {
		initialLimit = 20
	}
	if maxConcurrency < 0 {
		maxConcurrency = 1000
	}
	if smoothing < 0 || smoothing > 1.0 {
		smoothing = 1.0
	}
	if probeMultiplier <= 0 {
		probeMultiplier = 30
	}

	defaultLogFunc := functions.Log10RootFunction(0)
	if alphaFunc == nil {
		alphaFunc = func(limit int) int {
			return 3 * defaultLogFunc(limit)
		}
	}
	if betaFunc == nil {
		betaFunc = func(limit int) int {
			return 6 * defaultLogFunc(limit)
		}
	}
	if thresholdFunc == nil {
		thresholdFunc = func(limit int) int {
			return defaultLogFunc(limit)
		}
	}
	defaultLogFloatFunc := functions.Log10RootFloatFunction(0)
	if increaseFunc == nil {
		increaseFunc = func(limit float64) float64 {
			return limit + defaultLogFloatFunc(limit)
		}
	}
	if decreaseFunc == nil {
		decreaseFunc = func(limit float64) float64 {
			return limit - defaultLogFloatFunc(limit)
		}
	}

	return &VegasLimit{
		estimatedLimit: float64(initialLimit),
		maxLimit: maxConcurrency,
		alphaFunc: alphaFunc,
		betaFunc: betaFunc,
		thresholdFunc: thresholdFunc,
		increaseFunc: increaseFunc,
		decreaseFunc: decreaseFunc,
		smoothing: smoothing,
		probeMultipler: probeMultiplier,
		probeCountdown: nextVegasProbeCountdown(probeMultiplier, float64(initialLimit)),
		rttSampleListener: registry.RegisterDistribution(core.METRIC_MIN_RTT),
		registry: registry,
		logger: logger,
	}

}

// LimitProbeDisabled represents the disabled value for probing.
const LimitProbeDisabled = -1

func nextVegasProbeCountdown(probeMultiplier int, estimatedLimit float64) int {
	maxRange := int(float64(probeMultiplier) * estimatedLimit) / 2
	return rand.Intn(maxRange) + maxRange // return roughly [maxVal / 2, maxVal]
}

func (l *VegasLimit) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return int(l.estimatedLimit)
}

func (l *VegasLimit) Update(sample core.SampleWindow) {
	rtt := sample.CandidateRTTNanoseconds()
	if rtt <= 0 {
		panic(fmt.Sprintf("rtt must be > 0, got %d", rtt))
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.probeCountdown != LimitProbeDisabled {
		l.probeCountdown--
		if l.probeCountdown <= 0 {
			l.logger.Debugf("probe MinRTT %d", rtt / 1e6)
			l.probeCountdown = nextVegasProbeCountdown(l.probeMultipler, l.estimatedLimit)
			l.rttNoLoad = rtt
			return
		}
	}

	if l.rttNoLoad == 0 || rtt < l.rttNoLoad {
		l.logger.Debugf("New MinRTT %d", rtt / 1e6)
		l.rttNoLoad = rtt
		return
	}

	l.rttSampleListener.AddSample(float64(l.rttNoLoad))
	l.updateEstimatedLimit(sample, rtt)
}

func (l *VegasLimit) updateEstimatedLimit(sample core.SampleWindow, rtt int64) {
	queueSize := int(math.Ceil(l.estimatedLimit * (1 - float64(l.rttNoLoad) / float64(rtt))))

	var newLimit float64
	// Treat any drop (i.e timeout) as needing to reduce the limit
	if sample.DidDrop() {
		newLimit = l.decreaseFunc(l.estimatedLimit)
	} else if float64(sample.MaxInFlight()) * 2 < l.estimatedLimit {
		// Prevent upward drift if not close to the limit
		return
	} else {
		alpha := l.alphaFunc(int(l.estimatedLimit))
		beta := l.betaFunc(int(l.estimatedLimit))
		threshold := l.thresholdFunc(int(l.estimatedLimit))

		if queueSize < threshold {
			// Aggressive increase when no queuing
			newLimit = l.estimatedLimit + float64(beta)
		} else if queueSize < alpha {
			// Increase the limit if queue is still manageable
			newLimit = l.increaseFunc(l.estimatedLimit)
		} else if queueSize > beta {
			// Detecting latency so decrease
			newLimit = l.decreaseFunc(l.estimatedLimit)
		} else {
			// otherwise we're within he sweet spot so nothing to do
			return
		}
	}

	newLimit = math.Max(1, math.Min(float64(l.maxLimit), newLimit))
	newLimit = (1 - l.smoothing) * l.estimatedLimit + l.smoothing * newLimit

	if int(newLimit) != int(l.estimatedLimit) && l.logger.IsDebugEnabled() {
		l.logger.Debugf("New limit=%d, minRTT=%d ms, winRTT=%d ms, queueSize=%d",
			int(newLimit), l.rttNoLoad / 1e6, rtt / 1e6, queueSize)
	}

	l.estimatedLimit = newLimit
}

func (l *VegasLimit) RTTNoLoad() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rttNoLoad
}

func (l VegasLimit) String() string {
	return fmt.Sprintf("VegasLimit{limit=%d, rttNoLoad=%d ms}",
		l.EstimatedLimit(), l.RTTNoLoad())
}