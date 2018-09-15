package limit

import (
	"github.com/platinummonkey/go-concurrency-limits/measurements"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAIMDLimit(t *testing.T) {
	t.Run("Default", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("IncreaseOnSuccess", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 1).Nanoseconds(), 10))
		asrt.Equal(11, l.EstimatedLimit())
	})

	t.Run("DecreaseOnDrops", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := NewAIMDLimit(10, 0.9)
		l.Update(measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(1))
		asrt.Equal(9, l.EstimatedLimit())
	})
}
