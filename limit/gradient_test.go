package limit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

func TestGradientLimit(t *testing.T) {
	t.Parallel()
	t.Run("nextProbeInterval", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		asrt.Equal(LimitProbeDisabled, nextProbeCountdown(LimitProbeDisabled))
		asrt.True(nextProbeCountdown(1) > 0)
	})

	t.Run("Default", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewGradientLimitWithRegistry(
			0,
			0,
			0,
			-1,
			nil,
			-1,
			0,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)

		asrt.Equal(50, l.EstimatedLimit())
		asrt.Equal(int64(0), l.RTTNoLoad())
		asrt.Equal("GradientLimit{limit=50, rttNoLoad=0 ms}", l.String())
	})

	t.Run("panics with invalid RTT time", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewGradientLimitWithRegistry(
			0,
			0,
			0,
			-1,
			nil,
			-1,
			0,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		f := func() {
			l.OnSample(measurements.NewImmutableSampleWindow(-1, -1, 0, 0, 0, false))
		}
		asrt.Panics(f, "expected to panic with minRTT < 0")
	})

	t.Run("OnSample", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewGradientLimitWithRegistry(
			0,
			0,
			0,
			-1,
			nil,
			-1,
			0,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		// nothing should change
		m := measurements.NewDefaultImmutableSampleWindow()
		l.OnSample(m)
		asrt.Equal(50, l.EstimatedLimit())

		// dropped samples cut off limit, smoothed down
		m = m.AddDroppedSample(-1, 1)
		l.OnSample(m)
		asrt.Equal(45, l.EstimatedLimit())

		// test new sample shouldn't grow with current conditions
		m = measurements.NewDefaultImmutableSampleWindow().AddSample(-1, 0, 5)
		asrt.Equal(45, l.EstimatedLimit())

		// drain down pretty far
		for i := 0; i < 100; i++ {
			l.OnSample(measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(-1, 1))
		}
		asrt.Equal(4, l.EstimatedLimit())

		// slowly grow back up
		m = measurements.NewDefaultImmutableSampleWindow()
		for i := 0; i < 100; i++ {
			m = m.AddSample(-1, 1, 5)
			l.OnSample(m)
		}
		asrt.Equal(16, l.EstimatedLimit())
	})
}
