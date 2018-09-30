package limit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
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
			"test",
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

	t.Run("OnSample", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewGradientLimitWithRegistry(
			"test",
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
		listener := testNotifyListener{}
		l.NotifyOnChange(listener.updater())
		// nothing should change
		l.OnSample(0, 10, 1, false)
		asrt.Equal(50, l.EstimatedLimit())

		// dropped samples cut off limit, smoothed down
		l.OnSample(10, 1, 1, true)
		asrt.Equal(45, l.EstimatedLimit())
		asrt.Equal(45, listener.changes[0])

		// test new sample shouldn't grow with current conditions
		l.OnSample(20, 10, 5, false)
		asrt.Equal(45, l.EstimatedLimit())

		// drain down pretty far
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i*10+30), 0, 0, true)
		}
		asrt.Equal(4, l.EstimatedLimit())

		// slowly grow back up
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i*10+3030), 1, 5, false)
		}
		asrt.Equal(16, l.EstimatedLimit())
	})
}
