package limit

import (
	"github.com/platinummonkey/go-concurrency-limits/measurements"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFixedLimit(t *testing.T) {
	asrt := assert.New(t)
	l := NewFixedLimit(10)
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 10))
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 100))
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(100))
	asrt.Equal(10, l.EstimatedLimit())
}
