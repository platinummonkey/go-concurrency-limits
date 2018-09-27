package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExponentialAverageMeasurement(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m := NewExponentialAverageMeasurement(100, 10)

	expected := []float64{10, 10.5, 11, 11.5, 12, 12.5, 13, 13.5, 14, 14.5}
	for i := 0; i < 10; i++ {
		result, _ := m.Add(float64(i + 10))
		asrt.Equal(expected[i], result)
	}

	m.Add(100)
	asrt.InDelta(float64(16.19), m.Get(), 0.01)

	m.Update(func(value float64) float64 {
		return value - 1
	})
	asrt.InDelta(float64(15.19), m.Get(), 0.01)
}
