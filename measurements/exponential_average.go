package measurements

import (
	"fmt"
	"math"
	"sync"
)

// ExponentialWarmUpFunction describes a warmup function
type ExponentialWarmUpFunction func(currentValue, newSampleValue float64) float64

// ExponentialAverageMeasurement is an exponential average measurement implementation.
type ExponentialAverageMeasurement struct {
	value        float64
	window       int
	warmupWindow int
	warmupFunc   ExponentialWarmUpFunction
	count        int

	mu sync.RWMutex
}

// NewExponentialAverageMeasurement will create a new ExponentialAverageMeasurement
func NewExponentialAverageMeasurement(
	window int,
	warmupWindow int,
	warmupFunc func(currentValue, newSampleValue float64) float64,
) *ExponentialAverageMeasurement {
	if warmupFunc == nil {
		warmupFunc = ExponentialWarmUpFunction(func(currentValue, newSampleValue float64) float64 {
			return math.Min(currentValue, newSampleValue)
		})
	}

	return &ExponentialAverageMeasurement{
		window:       window,
		warmupWindow: warmupWindow,
		warmupFunc:   warmupFunc,
	}
}

// Add a single sample and update the internal state.
func (m *ExponentialAverageMeasurement) Add(value float64) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.count == 0 {
		m.count++
		m.value = value
	} else if m.count < m.warmupWindow {
		m.count++
		m.value = m.warmupFunc(m.value, value)
	} else {
		f := factor(m.window)
		m.value = m.value*(1-f) + value*f
	}
	return m.value, true
}

// Get the current value.
func (m *ExponentialAverageMeasurement) Get() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.value
}

// Reset the internal state as if no samples were ever added.
func (m *ExponentialAverageMeasurement) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = 0
	m.count = 0
}

// Update will update the value given an operation function
func (m *ExponentialAverageMeasurement) Update(operation func(value float64) float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = operation(m.value)
}

func factor(n int) float64 {
	return 2.0 / float64(n+1)
}

func (m *ExponentialAverageMeasurement) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf(
		"ExponentialAverageMeasurement{value=%0.5f, count=%d, window=%d, warmupWindow=%d}",
		m.value, m.count, m.window, m.warmupWindow,
	)
}
