package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit/functions"
	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

func createVegasLimit() *VegasLimit {
	return NewVegasLimitWithRegistry(
		10,
		20,
		1.0,
		functions.FixedQueueSizeFunc(3),
		functions.FixedQueueSizeFunc(6),
		nil,
		nil,
		nil,
		0,
		NoopLimitLogger{},
		core.EmptyMetricRegistryInstance)
}

func TestVegasLimit(t *testing.T) {
	t.Parallel()

	t.Run("NewDefaultVegasLimit", func(t2 *testing.T) {
		t2.Parallel()
		l := NewDefaultVegasLimit(NoopLimitLogger{}, core.EmptyMetricRegistryInstance)
		assert.Equal(t2, 20, l.EstimatedLimit())
	})

	t.Run("NewDefaultVegasLimitWithLimit", func(t2 *testing.T) {
		t2.Parallel()
		l := NewDefaultVegasLimitWithLimit(5, NoopLimitLogger{}, core.EmptyMetricRegistryInstance)
		assert.Equal(t2, 5, l.EstimatedLimit())
	})

	t.Run("InitialLimit", func(t2 *testing.T) {
		t2.Parallel()
		l := createVegasLimit()
		assert.Equal(t2, l.EstimatedLimit(), 10)
		assert.Equal(t2, l.RTTNoLoad(), int64(0))
		assert.Equal(t2, "VegasLimit{limit=10, rttNoLoad=0 ms}", l.String())
	})

	t.Run("IncreaseLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := createVegasLimit()
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 10))
		asrt.Equal(10, l.EstimatedLimit())
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 11))
		asrt.Equal(16, l.EstimatedLimit())
	})

	t.Run("DecreaseLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := createVegasLimit()
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 10))
		asrt.Equal(10, l.EstimatedLimit())
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 50).Nanoseconds(), 11))
		asrt.Equal(9, l.EstimatedLimit())
	})

	t.Run("NoChangeIfWithinThresholds", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := createVegasLimit()
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 10))
		asrt.Equal(10, l.EstimatedLimit())
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 14).Nanoseconds(), 14))
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("DecreaseSmoothing", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			100,
			200,
			0.5,
			nil,
			nil,
			nil,
			nil,
			func(estimatedLimit float64) float64 {
				return estimatedLimit / 2.0
			},
			0,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance)

		// Pick up first min-rtt
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 100))
		asrt.Equal(100, l.EstimatedLimit())

		// First decrease
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 20).Nanoseconds(), 100))
		asrt.Equal(75, l.EstimatedLimit())

		// Second decrease
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 20).Nanoseconds(), 100))
		asrt.Equal(56, l.EstimatedLimit())
	})

	t.Run("DecreaseWithoutSmoothing", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			100,
			200,
			-1,
			nil,
			nil,
			nil,
			nil,
			func(estimatedLimit float64) float64 {
				return estimatedLimit / 2.0
			},
			0,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance)

		// Pick up first min-rtt
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 100))
		asrt.Equal(100, l.EstimatedLimit())

		// First decrease
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 20).Nanoseconds(), 100))
		asrt.Equal(50, l.EstimatedLimit())

		// Second decrease
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 20).Nanoseconds(), 100))
		asrt.Equal(25, l.EstimatedLimit())
	})
}
