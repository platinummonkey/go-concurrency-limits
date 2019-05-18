package measurements

import (
	"math"
	"sync"
)

type SimpleMovingVariance struct {
	average  *SimpleExponentialMovingAverage
	variance *SimpleExponentialMovingAverage

	stdev      float64 // square root of the estimated variance
	normalized float64 // (signal - mean) / stdev

	mu sync.RWMutex
}

func NewSimpleMovingVariance(
	alphaAverage float64,
	alphaVariance float64,
) (*SimpleMovingVariance, error) {
	movingAverage, err := NewSimpleExponentialMovingAverage(alphaAverage)
	if err != nil {
		return nil, err
	}
	variance, err := NewSimpleExponentialMovingAverage(alphaVariance)
	if err != nil {
		return nil, err
	}
	return &SimpleMovingVariance{
		average:  movingAverage,
		variance: variance,
	}, nil
}

func (m *SimpleMovingVariance) Add(value float64) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	if m.average.seenSamples > 0 {
		m.variance.Add(math.Pow(value-m.average.Get(), 2))
	}
	m.average.Add(value)

	mean := m.average.Get()
	variance := m.variance.Get()
	stdev := math.Sqrt(variance)
	normalized := m.normalized
	if stdev != 0 {
		// edge case
		normalized = (value - mean) / stdev
	}
	//fmt.Printf("\tMV add: value=%0.5f mean=%0.5f variance=%0.5f stdev=%0.5f normalized=%0.5f\n", value, mean, variance, stdev, normalized)
	if stdev != m.stdev || normalized != m.normalized {
		changed = true
	}
	m.stdev = stdev
	m.normalized = normalized
	return stdev, changed
}

func (m *SimpleMovingVariance) Get() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.variance.Get()
}

func (m *SimpleMovingVariance) Reset() {
	m.mu.Lock()
	m.average.Reset()
	m.variance.Reset()
	m.stdev = 0
	m.normalized = 0
	m.mu.Unlock()
}

func (m *SimpleMovingVariance) Update(operation func(value float64) float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stdev = operation(m.variance.Get())
}
