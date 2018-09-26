package limit

import (
	"fmt"
	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
	"math"
	"sync"
	"time"
)

// WindowedLimit implements a windowed limit
type WindowedLimit struct {
	minWindowTime int64 // Minimum window duration for sampling a new minRtt
	maxWindowTime int64 // Maximum window duration for sampling a new minRtt
	nextUpdateTime int64 // End time for the sampling window at which point the limit should be updated
	windowSize int32 // Minimum sampling window size for finding a new minimum rtt
	minRTTThreshold int64

	delegate core.Limit
	sample *measurements.ImmutableSampleWindow

	mu sync.RWMutex
}

const (
	defaultWindowedMinWindowTime = 1e9 // 1 second
	defaultWindowedMaxWindowTime = 1e9 // 1 second
	defaultWindowedMinRTTThreshold = 1e8 // 100 microseconds
	defaultWindowedWindowSize = 10
)

// NewDefaultWindowedLimit will create a new default WindowedLimit
func NewDefaultWindowedLimit(
	delegate core.Limit,
) (*WindowedLimit, error) {
	return NewWindowedLimit(
		defaultWindowedMinWindowTime,
		defaultWindowedMaxWindowTime,
		defaultWindowedWindowSize,
		defaultWindowedMinRTTThreshold,
		delegate,
	)
}


// NewWindowedLimit will create a new WindowedLimit
func NewWindowedLimit(
	minWindowTime int64,
	maxWindowTime int64,
	windowSize int32,
	minRTTThreshold int64,
	delegate core.Limit,
) (*WindowedLimit, error) {
	if minWindowTime < (time.Duration(100)*time.Millisecond).Nanoseconds() {
		return nil, fmt.Errorf("minWindowTime must be >= 100 ms")
	}

	if maxWindowTime < (time.Duration(100)*time.Millisecond).Nanoseconds() {
		return nil, fmt.Errorf("maxWindowTime must be >= 100 ms")
	}

	if windowSize < 10 {
		return nil, fmt.Errorf("windowSize must be >= 10 ms")
	}

	if delegate == nil {
		return nil, fmt.Errorf("delegate must be specified")
	}

	return &WindowedLimit{
		minWindowTime: minWindowTime,
		maxWindowTime: maxWindowTime,
		nextUpdateTime: 0,
		windowSize: windowSize,
		minRTTThreshold: minRTTThreshold,
		delegate: delegate,
		sample: measurements.NewDefaultImmutableSampleWindow(),
	}, nil

}

// EstimatedLimit returns the current estimated limit.
func (l *WindowedLimit) EstimatedLimit() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.delegate.EstimatedLimit()
}

// @sample Data from the last sampling window such as RTT.
func (l *WindowedLimit) OnSample(sample core.SampleWindow) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := sample.(*measurements.ImmutableSampleWindow)
	endTime := s.StartTimeNanoseconds() + s.CandidateRTTNanoseconds()
	if s.CandidateRTTNanoseconds() < l.minRTTThreshold {
		return
	}

	if s.DidDrop() {
		l.sample = l.sample.AddDroppedSample(-1, s.MaxInFlight())
	} else {
		l.sample = l.sample.AddSample(-1, s.CandidateRTTNanoseconds(), s.MaxInFlight())
	}

	if endTime > l.nextUpdateTime && l.isWindowReady(sample) {
		current := l.sample
		l.sample = measurements.NewDefaultImmutableSampleWindow()
		l.nextUpdateTime = endTime + minInt64(maxInt64(current.CandidateRTTNanoseconds() * 2, l.minWindowTime), l.maxWindowTime)
		l.delegate.OnSample(measurements.NewImmutableSampleWindow(
			sample.StartTimeNanoseconds(),
			current.AverageRTTNanoseconds(),
			0,
			current.MaxInFlight(),
			current.SampleCount(),
			current.DidDrop(),
		))
	}
}

func (l *WindowedLimit) isWindowReady(sample core.SampleWindow) bool {
	return sample.CandidateRTTNanoseconds() < int64(math.MaxInt64) && int32(sample.SampleCount()) > l.windowSize
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

