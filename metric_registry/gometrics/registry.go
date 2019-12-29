// Package gometrics implements the metric registry interface for a gometrics provider.
package gometrics

import (
	"fmt"
	"strings"
	"sync"
	"time"

	gometrics "github.com/rcrowley/go-metrics"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

const defaultMetricPrefix = "limiter."
const defaultPollFrequency = time.Second * 5

type metricSampleListener struct {
	distribution gometrics.Histogram
	timer        gometrics.Timer
	counter      gometrics.Counter
	id           string
	metricType   uint8
}

// AddSample will add a sample metric to the listener
func (l *metricSampleListener) AddSample(value float64, tags ...string) {
	switch l.metricType {
	case 0: // distribution
		l.distribution.Update(int64(value))
	case 1: // timing
		// value is in epoch milliseconds
		l.timer.Update(time.Duration(value) * time.Millisecond)
	case 2: // count
		l.counter.Inc(int64(value))
	default:
		// unsupported
	}
}

type gometricsMetricPoller struct {
	supplier core.MetricSupplier
	id       string
	tags     []string
}

func (p *gometricsMetricPoller) poll() (string, float64, []string, bool) {
	val, ok := p.supplier()
	return p.id, val, p.tags, ok
}

// MetricRegistry will implements a MetricRegistry for sending metrics via go-metrics with any reporter.
type MetricRegistry struct {
	registry            gometrics.Registry
	prefix              string
	pollFrequency       time.Duration
	registeredGauges    map[string]*gometricsMetricPoller
	registeredListeners map[string]*metricSampleListener

	started bool
	stopper chan bool
	mu      sync.Mutex
	wg      sync.WaitGroup
}

// NewGoMetricsMetricRegistry will create a new Datadog MetricRegistry.
// This registry reports metrics to datadog using the datadog dogstatsd forwarding.
func NewGoMetricsMetricRegistry(
	registry gometrics.Registry,
	addr string,
	prefix string,
	pollFrequency time.Duration,
) (*MetricRegistry, error) {
	if prefix == "" {
		prefix = defaultMetricPrefix
	}
	if !strings.HasSuffix(prefix, ".") {
		prefix = prefix + "."
	}

	if pollFrequency == 0 {
		pollFrequency = defaultPollFrequency
	}

	if registry == nil {
		return nil, fmt.Errorf("registry required")
	}

	return &MetricRegistry{
		registry:      registry,
		prefix:        prefix,
		pollFrequency: pollFrequency,
		stopper:       make(chan bool, 1),
	}, nil
}

// Start will start the metric registry polling
func (r *MetricRegistry) Start() {
	r.mu.Lock()
	if !r.started {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.run()
		}()
	}
	r.mu.Unlock()
}

func (r *MetricRegistry) run() {
	ticker := time.NewTicker(r.pollFrequency)
	for {
		select {
		case <-r.stopper:
			return
		case <-ticker.C:
			// poll the gauges
			r.mu.Lock()
			for _, g := range r.registeredGauges {
				metricSuffix, value, _, ok := g.poll()
				if ok {
					m := gometrics.GetOrRegisterGaugeFloat64(r.prefix+metricSuffix, r.registry)
					m.Update(value)
				}
			}
			r.mu.Unlock()
		}
	}
}

// Stop will gracefully stop the registry
func (r *MetricRegistry) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.stopper <- true
	r.wg.Wait()
	r.started = false
	r.mu.Unlock()
}

// RegisterDistribution will register a distribution sample to this registry
func (r *MetricRegistry) RegisterDistribution(
	ID string,
	tags ...string,
) core.MetricSampleListener {
	if strings.HasPrefix(ID, ".") {
		ID = strings.TrimPrefix(ID, ".")
	}

	// only add once
	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	r.registeredListeners[ID] = &metricSampleListener{
		distribution: gometrics.GetOrRegisterHistogram(
			r.prefix+ID,
			r.registry,
			gometrics.NewUniformSample(100),
		),
		metricType: 0,
		id:         r.prefix + ID,
	}

	return r.registeredListeners[ID]
}

// RegisterTiming will register a timing distribution sample to this registry
func (r *MetricRegistry) RegisterTiming(
	ID string,
	tags ...string,
) core.MetricSampleListener {
	if strings.HasPrefix(ID, ".") {
		ID = strings.TrimPrefix(ID, ".")
	}

	// only add once
	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	r.registeredListeners[ID] = &metricSampleListener{
		timer: gometrics.GetOrRegisterTimer(
			r.prefix+ID,
			r.registry,
		),
		metricType: 1,
		id:         r.prefix + ID,
	}

	return r.registeredListeners[ID]
}

// RegisterCount will register a count sample to this registry
func (r *MetricRegistry) RegisterCount(
	ID string,
	tags ...string,
) core.MetricSampleListener {
	if strings.HasPrefix(ID, ".") {
		ID = strings.TrimPrefix(ID, ".")
	}

	// only add once
	if l, ok := r.registeredListeners[ID]; ok {
		return l
	}

	r.registeredListeners[ID] = &metricSampleListener{
		counter: gometrics.GetOrRegisterCounter(
			r.prefix+ID,
			r.registry,
		),
		metricType: 2,
		id:         r.prefix + ID,
	}

	return r.registeredListeners[ID]
}

// RegisterGauge will register a gauge sample to this registry
func (r *MetricRegistry) RegisterGauge(
	ID string,
	supplier core.MetricSupplier,
	tags ...string,
) {
	if strings.HasPrefix(ID, ".") {
		ID = strings.TrimPrefix(ID, ".")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// only add once
	if _, ok := r.registeredGauges[ID]; ok {
		return
	}

	r.registeredGauges[ID] = &gometricsMetricPoller{
		supplier: supplier,
		id:       ID,
		tags:     tags,
	}
}
