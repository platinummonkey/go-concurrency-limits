package measurements

import (
	"sync"
)

// MaximumMeasurement implements a maximum value measurement
type MaximumMeasurement struct {
	value float64
	mu sync.RWMutex
}

func (m *MaximumMeasurement) Add(sample float64) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldValue := float64(m.value)
	if oldValue == 0.0 || sample > oldValue {
		m.value = sample
	}
	return m.value, oldValue == m.value
}

func (m *MaximumMeasurement) Get() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.value
}

func (m *MaximumMeasurement) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value = 0.0
}



