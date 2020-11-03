package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit/functions"
)

func createVegasLimit() *VegasLimit {
	return NewVegasLimitWithRegistry(
		"test",
		10,
		nil,
		20,
		1.0,
		0,
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
		l := NewDefaultVegasLimit("test", NoopLimitLogger{}, nil)
		assert.Equal(t2, 20, l.EstimatedLimit())
	})

	t.Run("NewDefaultVegasLimitWithLimit", func(t2 *testing.T) {
		t2.Parallel()
		l := NewDefaultVegasLimitWithLimit("test", 5, NoopLimitLogger{}, nil)
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
		listener := testNotifyListener{}
		l.NotifyOnChange(listener.updater())
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
		asrt.Equal(10, l.EstimatedLimit())
		l.OnSample(10, (time.Millisecond * 10).Nanoseconds(), 11, false)
		asrt.Equal(16, l.EstimatedLimit())
		asrt.Equal(16, listener.changes[0])
	})

	t.Run("DecreaseLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := createVegasLimit()
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
		asrt.Equal(10, l.EstimatedLimit())
		l.OnSample(10, (time.Millisecond * 50).Nanoseconds(), 11, false)
		asrt.Equal(9, l.EstimatedLimit())
	})

	t.Run("NoChangeIfWithinThresholds", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := createVegasLimit()
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
		asrt.Equal(10, l.EstimatedLimit())
		l.OnSample(10, (time.Millisecond * 14).Nanoseconds(), 14, false)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("DecreaseSmoothing", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			"test",
			100,
			nil,
			200,
			0.5,
			0,
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
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 100, false)
		asrt.Equal(100, l.EstimatedLimit())

		// First decrease
		l.OnSample(10, (time.Millisecond * 20).Nanoseconds(), 100, false)
		asrt.Equal(75, l.EstimatedLimit())

		// Second decrease
		l.OnSample(20, (time.Millisecond * 20).Nanoseconds(), 100, false)
		asrt.Equal(56, l.EstimatedLimit())
	})

	t.Run("DecreaseWithoutSmoothing", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			"test",
			100,
			nil,
			200,
			-1,
			0,
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
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 100, false)
		asrt.Equal(100, l.EstimatedLimit())

		// First decrease
		l.OnSample(10, (time.Millisecond * 20).Nanoseconds(), 100, false)
		asrt.Equal(50, l.EstimatedLimit())

		// Second decrease
		l.OnSample(20, (time.Millisecond * 20).Nanoseconds(), 100, false)
		asrt.Equal(25, l.EstimatedLimit())
	})

	t.Run("DecreaseLimitWithBufferFactor", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			"test",
			10,
			nil,
			200,
			-1,
			1.0,
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
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
		asrt.Equal(10, l.EstimatedLimit())

		// First decrease
		l.OnSample(10, (time.Millisecond * 50).Nanoseconds(), 11, false)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("NoChangeIfWithinThresholdsWithBuffer", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewVegasLimitWithRegistry(
			"test",
			10,
			nil,
			200,
			-1,
			1.0,
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
		l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
		asrt.Equal(10, l.EstimatedLimit())

		// First decrease
		l.OnSample(10, (time.Millisecond * 5).Nanoseconds(), 14, false)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("PauseUpdateWhenProbeWithBuffer", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		listener := testNotifyListener{}
		l := NewVegasLimitWithRegistry(
			"test",
			10,
			nil,
			200,
			-1,
			1.0,
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

		l.NotifyOnChange(listener.updater())

		for i := 1; i < 600; i++ {
			l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 100, false)
		}
		asrt.Equal(16, listener.changes[0])
		asrt.Equal(22, listener.changes[1])
		asrt.Equal(28, listener.changes[2])
	})

}
