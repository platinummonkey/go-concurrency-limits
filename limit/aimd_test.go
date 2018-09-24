package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

func TestAIMDLimit(t *testing.T) {
	t.Parallel()

	t.Run("DefaultAIMD", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewDefaultAIMLimit()
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("Default", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		asrt.Equal(10, l.EstimatedLimit())
		asrt.Equal(0.9, l.BackOffRatio())
	})

	t.Run("IncreaseOnSuccess", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 1).Nanoseconds(), 10))
		asrt.Equal(11, l.EstimatedLimit())
	})

	t.Run("DecreaseOnDrops", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(1))
		asrt.Equal(9, l.EstimatedLimit())
	})

	t.Run("String", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		asrt.Equal("AIMDLimit{limit=10, backOffRatio=0.9000}", l.String())
	})
}
