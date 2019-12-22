package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowlessMovingPercentile(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	m, err := NewWindowlessMovingPercentile(0.9, 0.01, 0.05, 0.05)
	asrt.NoError(err)
	asrt.NotNil(m)
	asrt.Equal(float64(0.0), m.Get())
	for i := 0; i < 10; i++ {
		m.Add(100)
	}
	asrt.Equal(float64(100), m.Get())
	m.Add(99)
	for i := 0; i < 10; i++ {
		m.Add(1000)
	}
	m.Add(0.1)
	asrt.Equal(520, int(m.Get()))

	m.Reset()
	asrt.Equal(float64(0), m.Get())
	m.Update(func(value float64) float64 {
		return 1.0
	})
	asrt.Equal(float64(1.0), m.Get())
}
