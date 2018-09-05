package limiter

import (
	"fmt"
	"math"
)

// ImmutableSampleWindow is used to track immutable samples atomically.
type ImmutableSampleWindow struct {
	minRTT int64
	maxInFlight int
	sampleCount int
	sum int64
	didDrop bool
}

func NewImmutableSampleWindow(
	minRTT int64,
	sum int64,
	maxInFlight int,
	sampleCount int,
	didDrop bool,
) *ImmutableSampleWindow {
	if minRTT == 0 {
		minRTT = math.MaxInt64
	}
	return &ImmutableSampleWindow{
		minRTT: minRTT,
		sum: sum,
		maxInFlight: maxInFlight,
		sampleCount: sampleCount,
		didDrop: didDrop,
	}
}

// AddSample will create a new immutable sample for which to use.
func (s *ImmutableSampleWindow) AddSample(rtt int64, maxInFlight int) *ImmutableSampleWindow {
	minRTT := s.minRTT
	if rtt < s.minRTT {
		minRTT = rtt
	}
	if maxInFlight < s.maxInFlight {
		maxInFlight = s.maxInFlight
	}
	return NewImmutableSampleWindow(minRTT, s.sum + rtt, maxInFlight, s.sampleCount + 1, false)
}

// AddDroppedSample will create a new immutable sample that was dropped.
func (s *ImmutableSampleWindow) AddDroppedSample(maxInFlight int) *ImmutableSampleWindow {
	if maxInFlight < s.maxInFlight {
		maxInFlight = s.maxInFlight
	}
	return NewImmutableSampleWindow(s.minRTT, s.sum, maxInFlight, s.sampleCount, true)
}

func (s *ImmutableSampleWindow) CandidateRTTNanoseconds() int64 {
	return s.minRTT
}

func (s *ImmutableSampleWindow) AverageRTTNanoseconds() int64 {
	if s.sampleCount == 0 {
		return 0
	}
	return s.sum / int64(s.sampleCount)
}

func (s *ImmutableSampleWindow) MaxInFlight() int {
	return s.maxInFlight
}

func (s *ImmutableSampleWindow) SampleCount() int {
	return s.sampleCount
}

func (s *ImmutableSampleWindow) DidDrop() bool {
	return s.didDrop
}

func (s *ImmutableSampleWindow) String() string {
	return fmt.Sprintf(
		"ImmutableSampleWindow{minRTT=%d, averageRTT=%d, maxInFlight=%d, sampleCount=%d, didDrop=%t",
		s.minRTT, s.AverageRTTNanoseconds(), s.maxInFlight, s.sampleCount, s.didDrop)
}
