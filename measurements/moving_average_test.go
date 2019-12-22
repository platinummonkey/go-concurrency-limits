package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleExponentialMovingAverage(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m, err := NewSimpleExponentialMovingAverage(0.05)
	asrt.NoError(err)
	asrt.NotNil(m)

	asrt.Equal(float64(0), m.Get())
	m.Add(10)
	asrt.Equal(float64(10), m.Get())
	m.Add(11)
	asrt.Equal(float64(10.5), m.Get())
	m.Add(11)
	m.Add(11)
	asrt.Equal(float64(10.75), m.Get())

	m.Reset()
	asrt.Equal(float64(0), m.Get())
	m.Update(func(value float64) float64 {
		return 1.0
	})
	asrt.Equal(float64(1.0), m.Get())
}
