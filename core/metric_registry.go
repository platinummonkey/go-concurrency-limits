package core

// MetricSampleListener is a listener to receive samples for a distribution
type MetricSampleListener interface {
	// AddSample will add a sample metric to the listener
	AddSample(value float64, tags ...string)
}

// EmptyMetricSampleListener implements a sample listener that ignores everything.
type EmptyMetricSampleListener struct{}

// AddSample will add a metric sample to this listener
func (*EmptyMetricSampleListener) AddSample(value float64, tags ...string) {
	// noop
}

// MetricSupplier will return the supplied metric value
type MetricSupplier func() (value float64, ok bool)

// NewIntMetricSupplierWrapper will wrap a int-return value func to a supplier func
func NewIntMetricSupplierWrapper(s func() int) MetricSupplier {
	return MetricSupplier(func() (float64, bool) {
		val := s()
		return float64(val), true
	})
}

// NewFloat64MetricSupplierWrapper will wrap a int-return value func to a supplier func
func NewFloat64MetricSupplierWrapper(s func() float64) MetricSupplier {
	return MetricSupplier(func() (float64, bool) {
		val := s()
		return val, true
	})
}

// MetricRegistry is a simple abstraction for tracking metrics in the limiters.
type MetricRegistry interface {
	// RegisterDistribution will register a sample distribution.  Samples are added to the distribution via the returned
	// MetricSampleListener. Will reuse an existing MetricSampleListener if the distribution already exists.
	RegisterDistribution(ID string, tags ...string) MetricSampleListener

	// RegisterTiming will register a sample timing distribution.  Samples are added to the distribution via the
	// returned MetricSampleListener. Will reuse an existing MetricSampleListener if the distribution already exists.
	RegisterTiming(ID string, tags ...string) MetricSampleListener

	// RegisterCount will register a sample counter.  Samples are added to the counter via the returned
	// MetricSampleListener. Will reuse an existing MetricSampleListener if the counter already exists.
	RegisterCount(ID string, tags ...string) MetricSampleListener

	// RegisterGauge will register a gauge using the provided supplier.  The supplier will be polled whenever the gauge
	// value is flushed by the registry.
	RegisterGauge(ID string, supplier MetricSupplier, tags ...string)

	// Start will start the metric registry polling
	Start()

	// Stop will stop the metric registry polling
	Stop()
}

// EmptyMetricRegistry implements a void reporting metric registry
type EmptyMetricRegistry struct{}

// EmptyMetricRegistryInstance is a singleton empty metric registry instance.
var EmptyMetricRegistryInstance = &EmptyMetricRegistry{}

// RegisterDistribution will register a distribution sample to this registry
func (*EmptyMetricRegistry) RegisterDistribution(ID string, tags ...string) MetricSampleListener {
	return &EmptyMetricSampleListener{}
}

// RegisterTiming will register a timing distribution sample to this registry
func (*EmptyMetricRegistry) RegisterTiming(ID string, tags ...string) MetricSampleListener {
	return &EmptyMetricSampleListener{}
}

// RegisterCount will register a count sample to this registry
func (*EmptyMetricRegistry) RegisterCount(ID string, tags ...string) MetricSampleListener {
	return &EmptyMetricSampleListener{}
}

// RegisterGauge will register a gauge sample to this registry
func (*EmptyMetricRegistry) RegisterGauge(ID string, supplier MetricSupplier, tags ...string) {}

// Start will start the metric registry polling
func (*EmptyMetricRegistry) Start() {}

// Stop will stop the metric registry polling
func (*EmptyMetricRegistry) Stop() {}
