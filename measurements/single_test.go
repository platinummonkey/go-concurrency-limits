package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingleMeasurement(t *testing.T) {
	asrt := assert.New(t)
	m := SingleMeasurement{}
	asrt.Equal(float64(0.0), m.Get())

	m.Add(1)
	m.Add(2)
	asrt.Equal(float64(2.0), m.Get())

	m.Update(func(value float64) float64 {
		return value * 2
	})
	asrt.Equal(float64(4.0), m.Get())
	asrt.Equal("SingleMeasurement{value=4.00000}", m.String())

	m.Reset()
	asrt.Equal(float64(0), m.Get())
}
