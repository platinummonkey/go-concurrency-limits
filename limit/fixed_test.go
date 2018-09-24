package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

func TestFixedLimit(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	l := NewFixedLimit(10)
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 10))
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddSample((time.Millisecond * 10).Nanoseconds(), 100))
	asrt.Equal(10, l.EstimatedLimit())

	l.Update(measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(100))
	asrt.Equal(10, l.EstimatedLimit())

	asrt.Equal("FixedLimit{limit=10}", l.String())
}
