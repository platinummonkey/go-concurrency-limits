package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinimumMeasurement(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m := MinimumMeasurement{}
	asrt.Equal(float64(0.0), m.Get())
	m.Add(-1)
	asrt.Equal(float64(-1.0), m.Get())
	m.Add(0)
	asrt.Equal(float64(-1.0), m.Get())
	m.Reset()
	asrt.Equal(float64(0.0), m.Get())
	m.Update(func(value float64) float64 {
		return value - 1
	})
	asrt.Equal(float64(-1.0), m.Get())

	asrt.Equal("MinimumMeasurement{value=-1.00000}", m.String())
}
