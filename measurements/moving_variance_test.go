package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleMovingVariance(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m, err := NewSimpleMovingVariance(0.05, 0.05)
	asrt.NoError(err)
	asrt.NotNil(m)

	// initial condition
	asrt.Equal(float64(0), m.Get())
	// warmup first sample
	m.Add(10)
	asrt.Equal(float64(0), m.Get())
	// first variance reading is expected to be 1 here
	m.Add(11)
	asrt.Equal(float64(1), m.Get())
	m.Add(10)
	m.Add(11)
	asrt.InDelta(float64(0.5648), m.Get(), 0.00005)
	m.Add(20)
	m.Add(100)
	m.Add(30)
	asrt.InDelta(float64(1295.7841), m.Get(), 0.00005)
}
