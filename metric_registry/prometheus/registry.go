// Package prometheus implements the metric registry interface for a Prometheus provider.
package prometheus

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

const defaultMetricNamespace = "limiter"

type metricSampleListener struct {
	summary    prometheus.Summary
	counter    prometheus.Counter
	metricType uint8
}

// AddSample will add a sample metric to the listener.
func (l *metricSampleListener) AddSample(value float64, tags ...string) {
	switch l.metricType {
	case 0: // distribution
		l.summary.Observe(value)
	case 1: // timing (value is epoch milliseconds)
		l.summary.Observe(value)
	case 2: // count
		l.counter.Add(value)
	}
}

// MetricRegistry implements core.MetricRegistry using Prometheus.
//
// Distributions and timings are exposed as Summaries. Counts are exposed as
// Counters. Gauges are registered as GaugeFuncs — Prometheus scrapes them on
// demand, so no background polling goroutine is required; Start and Stop are
// no-ops.
//
// Tags passed to Register* methods are parsed as "key:value" pairs and
// attached to the metric as Prometheus const-labels.
type MetricRegistry struct {
	registerer          prometheus.Registerer
	namespace           string
	registeredListeners map[string]*metricSampleListener
	registeredGauges    map[string]prometheus.GaugeFunc

	mu sync.Mutex
}

// NewMetricRegistry creates a new Prometheus MetricRegistry that registers
// metrics with the default Prometheus registry.
//
//   - namespace is used as the Prometheus metric namespace (e.g. "limiter").
//     If empty, "limiter" is used.
func NewMetricRegistry(namespace string) *MetricRegistry {
	return NewMetricRegistryWithRegisterer(namespace, prometheus.DefaultRegisterer)
}

// NewMetricRegistryWithRegisterer creates a new Prometheus MetricRegistry that
// registers metrics with the provided prometheus.Registerer. Pass a
// prometheus.NewRegistry() here in tests to avoid polluting the global registry.
func NewMetricRegistryWithRegisterer(namespace string, registerer prometheus.Registerer) *MetricRegistry {
	if namespace == "" {
		namespace = defaultMetricNamespace
	}
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	return &MetricRegistry{
		registerer:          registerer,
		namespace:           namespace,
		registeredListeners: make(map[string]*metricSampleListener),
		registeredGauges:    make(map[string]prometheus.GaugeFunc),
	}
}

// sanitizeName converts a metric ID (which may contain dots) to a valid
// Prometheus metric name subsystem/name component by replacing dots with
// underscores.
func sanitizeName(id string) string {
	id = strings.TrimPrefix(id, ".")
	return strings.ReplaceAll(id, ".", "_")
}

// parseTags converts variadic "key:value" tag strings into prometheus.Labels.
// Entries that do not contain ":" are ignored.
func parseTags(tags []string) prometheus.Labels {
	labels := prometheus.Labels{}
	for _, t := range tags {
		parts := strings.SplitN(t, ":", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}
	return labels
}

// RegisterDistribution registers a sample distribution backed by a Prometheus
// Summary. Returns an existing listener if the ID was already registered.
func (r *MetricRegistry) RegisterDistribution(ID string, tags ...string) core.MetricSampleListener {
	r.mu.Lock()
	defer r.mu.Unlock()

	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	name := sanitizeName(ID)
	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:   r.namespace,
		Name:        name,
		Help:        "Distribution samples for " + ID,
		ConstLabels: parseTags(tags),
	})
	r.registerer.MustRegister(summary)

	l := &metricSampleListener{summary: summary, metricType: 0}
	r.registeredListeners[ID] = l
	return l
}

// RegisterTiming registers a timing distribution backed by a Prometheus Summary.
// Sample values are expected in milliseconds. Returns an existing listener if
// the ID was already registered.
func (r *MetricRegistry) RegisterTiming(ID string, tags ...string) core.MetricSampleListener {
	r.mu.Lock()
	defer r.mu.Unlock()

	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	name := sanitizeName(ID)
	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:   r.namespace,
		Name:        name + "_ms",
		Help:        "Timing samples (ms) for " + ID,
		ConstLabels: parseTags(tags),
	})
	r.registerer.MustRegister(summary)

	l := &metricSampleListener{summary: summary, metricType: 1}
	r.registeredListeners[ID] = l
	return l
}

// RegisterCount registers a counter backed by a Prometheus Counter. Returns an
// existing listener if the ID was already registered.
func (r *MetricRegistry) RegisterCount(ID string, tags ...string) core.MetricSampleListener {
	r.mu.Lock()
	defer r.mu.Unlock()

	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	name := sanitizeName(ID)
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   r.namespace,
		Name:        name + "_total",
		Help:        "Total count for " + ID,
		ConstLabels: parseTags(tags),
	})
	r.registerer.MustRegister(counter)

	l := &metricSampleListener{counter: counter, metricType: 2}
	r.registeredListeners[ID] = l
	return l
}

// RegisterGauge registers a gauge backed by a Prometheus GaugeFunc. The
// supplier is called each time Prometheus scrapes the metric — no polling
// goroutine is needed.
func (r *MetricRegistry) RegisterGauge(ID string, supplier core.MetricSupplier, tags ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.registeredGauges[ID]; ok {
		return
	}

	name := sanitizeName(ID)
	gauge := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace:   r.namespace,
			Name:        name,
			Help:        "Gauge for " + ID,
			ConstLabels: parseTags(tags),
		},
		func() float64 {
			val, ok := supplier()
			if !ok {
				return 0
			}
			return val
		},
	)
	r.registerer.MustRegister(gauge)
	r.registeredGauges[ID] = gauge
}

// Start is a no-op for the Prometheus registry. Prometheus uses a pull model
// so no background polling goroutine is needed.
func (r *MetricRegistry) Start() {}

// Stop is a no-op for the Prometheus registry.
func (r *MetricRegistry) Stop() {}
