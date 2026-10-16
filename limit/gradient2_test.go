package limit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

func TestGradient2Limit(t *testing.T) {
	t.Parallel()

	t.Run("Default", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewDefaultGradient2Limit("test", nil, nil)
		asrt.NotNil(l)

		asrt.Equal(20, l.EstimatedLimit())
		asrt.Equal("Gradient2Limit{limit=20}", l.String())
	})

	t.Run("OnSample", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l, err := NewGradient2Limit(
			"test",
			50,
			0,
			0,
			nil,
			-1,
			-1,
			NoopLimitLogger{},
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(l)
		listener := testNotifyListener{}
		l.NotifyOnChange(listener.updater())

		// nothing should change
		l.OnSample(0, 10, 1, false)
		asrt.Equal(50, l.EstimatedLimit())

		for i := 0; i < 25; i++ {
			l.OnSample(int64(i), 10*int64(i), i, false)
			asrt.Equal(50, l.EstimatedLimit())
		}

		// dropped samples cut off limit, smoothed down
		l.OnSample(25, 2500, 25, true)
		asrt.Equal(45, l.EstimatedLimit())
		asrt.Equal(45, listener.changes[0])

		// test new sample shouldn't grow too fast
		l.OnSample(26, 10, 5, false)
		asrt.Equal(45, l.EstimatedLimit())

		// drain down again
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i+27), 1000, i, true)
		}
		asrt.Equal(21, l.EstimatedLimit())

		// slowly grow back up
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i+127), 1, 1, false)
		}
		asrt.Equal(21, l.EstimatedLimit())
	})

	t.Run("OnSampleWithRTTTolerance", func(t2 *testing.T) {
		t2.Parallel()

		newLimit := func(tolerance float64) *Gradient2Limit {
			l, err := NewGradient2LimitWithRTTTolerance(
				"test",
				50,
				60,
				0,
				nil,
				-1,
				-1,
				tolerance,
				NoopLimitLogger{},
				core.EmptyMetricRegistryInstance,
			)
			assert.NoError(t2, err)
			return l
		}

		seed := func(l *Gradient2Limit) {
			// establish a stable long term RTT baseline of 1000ns;
			// inFlight stays above half the limit so the limit grows to the 60 cap
			for i := 0; i < 100; i++ {
				l.OnSample(int64(i), 1000, 100, false)
			}
		}

		t2.Run("DefaultToleranceReducesLimitOnModerateRTTIncrease", func(t3 *testing.T) {
			t3.Parallel()
			asrt := assert.New(t3)
			l := newLimit(1.0)
			seed(l)
			asrt.Equal(60, l.EstimatedLimit())

			// 1.5x RTT increase: without tolerance the gradient drops below 1.0
			for i := 0; i < 10; i++ {
				l.OnSample(int64(i+100), 1500, 100, false)
			}
			asrt.Less(l.EstimatedLimit(), 60)
		})

		t2.Run("ToleranceTwoAbsorbsModerateRTTIncrease", func(t3 *testing.T) {
			t3.Parallel()
			asrt := assert.New(t3)
			l := newLimit(2.0)
			seed(l)
			asrt.Equal(60, l.EstimatedLimit())

			// 1.5x RTT increase is within tolerance: gradient stays 1.0, limit holds at cap
			for i := 0; i < 10; i++ {
				l.OnSample(int64(i+100), 1500, 100, false)
			}
			asrt.Equal(60, l.EstimatedLimit())
		})

		t2.Run("ToleranceTwoStillCutsLimitOnSevereRTTIncrease", func(t3 *testing.T) {
			t3.Parallel()
			asrt := assert.New(t3)
			l := newLimit(2.0)
			seed(l)
			asrt.Equal(60, l.EstimatedLimit())

			// 4x RTT increase exceeds tolerance: gradient clamped to 0.5, limit shrinks
			for i := 0; i < 10; i++ {
				l.OnSample(int64(i+100), 4000, 100, false)
			}
			asrt.Less(l.EstimatedLimit(), 60)
		})
	})
}
