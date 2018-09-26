package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExponentialAverageMeasurement(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m := NewExponentialAverageMeasurement(10, 2)
	asrt.Equal(float64(0.0), m.Get())
	m.Add(1)
	asrt.Equal(float64(1.0), m.Get())
	m.Add(2)
	asrt.Equal(float64(1.5), m.Get())
	m.Add(15.5)
	asrt.InDelta(float64(4.04), m.Get(), 0.006)
	m.Update(func(value float64) float64 {
		return value - 1
	})
	asrt.InDelta(float64(3.04), m.Get(), 0.006)
}
