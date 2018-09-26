package measurements

import (
	"sync"
)

// MinimumMeasurement implements a minimum value measurement
type MinimumMeasurement struct {
	value float64
	mu    sync.RWMutex
}

// Add will compare the sample and save if it's the minimum value.
func (m *MinimumMeasurement) Add(sample float64) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldValue := float64(m.value)
	if oldValue == 0.0 || sample < oldValue {
		m.value = sample
	}
	return m.value, oldValue == m.value
}

// Get will return the current minimum value
func (m *MinimumMeasurement) Get() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.value
}

// Reset will reset the minimum value to 0.0
func (m *MinimumMeasurement) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = 0.0
}

// OnSample will update the value given an operation function
func (m *MinimumMeasurement) Update(operation func(value float64) float64) {
	m.mu.RLock()
	current := m.value
	m.mu.RUnlock()
	m.Add(operation(current))
}
