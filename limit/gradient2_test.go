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

		asrt.Equal(4, l.EstimatedLimit())
		asrt.Equal("Gradient2Limit{limit=4}", l.String())
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
			-1,
			-1,
			nil,
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

		for i := 0; i < 51; i++ {
			l.OnSample(int64(i), 100, 1, false)
			asrt.Equal(50, l.EstimatedLimit())
		}

		// dropped samples cut off limit, smoothed down
		l.OnSample(60, 100, 1, true)
		asrt.Equal(4, l.EstimatedLimit())
		asrt.Equal(4, listener.changes[0])

		// test new sample shouldn't grow too fast
		l.OnSample(20, 10, 5, false)
		asrt.Equal(6, l.EstimatedLimit())

		// drain down again
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i*10+30), 10, 1, true)
		}
		asrt.Equal(6, l.EstimatedLimit())

		// slowly grow back up
		for i := 0; i < 100; i++ {
			l.OnSample(int64(i*10+3030), 1, 5, false)
		}
		asrt.Equal(13, l.EstimatedLimit())
	})
}
