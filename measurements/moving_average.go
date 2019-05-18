package measurements

import (
	"fmt"
	"math"
	"sync"
)

type SimpleExponentialMovingAverage struct {
	alpha        float64
	initialAlpha float64
	minSamples   int
	seenSamples  int

	value float64

	mu sync.RWMutex
}

func NewSimpleExponentialMovingAverage(
	alpha float64,
) (*SimpleExponentialMovingAverage, error) {
	if alpha < 0 || alpha > 1 {
		return nil, fmt.Errorf("alpha must be [0, 1]")
	}
	minSamples := int(math.Trunc(math.Ceil(1 / alpha)))
	return &SimpleExponentialMovingAverage{
		alpha:        alpha,
		initialAlpha: alpha,
		minSamples:   minSamples,
	}, nil
}

func (m *SimpleExponentialMovingAverage) Add(value float64) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.add(value)
}

func (m *SimpleExponentialMovingAverage) add(value float64) (float64, bool) {
	changed := false
	if m.seenSamples < m.minSamples {
		m.seenSamples++
	}
	var alpha float64
	if m.seenSamples >= m.minSamples {
		alpha = m.alpha
	} else {
		alpha = 1 / float64(m.seenSamples)
	}
	newValue := (1-alpha)*m.value + alpha*value
	if newValue != m.value {
		changed = true
	}
	m.value = newValue
	return m.value, changed
}

func (m *SimpleExponentialMovingAverage) Get() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.value
}

func (m *SimpleExponentialMovingAverage) Reset() {
	m.mu.Lock()
	m.seenSamples = 0
	m.value = 0
	m.alpha = m.initialAlpha
	m.mu.Unlock()
}

func (m *SimpleExponentialMovingAverage) Update(operation func(value float64) float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newValue, _ := m.add(m.value)
	m.value = operation(newValue)
}
