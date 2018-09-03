package core


// MetricSampleListener is a listener to receive samples for a distribution
type MetricSampleListener interface {
	//
	AddSample(value float64)
}

// EmptydMetricSampleListener implements a sample listener that ignores everything.
type EmptyMetricSampleListener struct {}

func (*EmptyMetricSampleListener) AddSample(value float64) {
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
	RegisterDistribution(ID string, tagNameValuePairs ...string) MetricSampleListener

	// RegisterGauge will register a gauge using the provided supplier.  The supplier will be polled whenever the gauge
	// value is flushed by the registry.
	RegisterGauge(ID string, supplier MetricSupplier, tagNameValuePairs ...string)

}


// EmptyMetricRegistry implements a void reporting metric registry
type EmptyMetricRegistry struct {}

// EmptyMetricRegistryInstance is a singleton empty metric registry instance.
var EmptyMetricRegistryInstance = &EmptyMetricRegistry{}

func (*EmptyMetricRegistry) RegisterDistribution(ID string, tagNameValuePairs ...string) MetricSampleListener {
	return &EmptyMetricSampleListener{}
}

func (*EmptyMetricRegistry) RegisterGauge(ID string, supplier MetricSupplier, tagNameValuePairs ...string) {
	// noop
}


