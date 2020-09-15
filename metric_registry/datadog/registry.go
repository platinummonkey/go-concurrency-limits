// Package datadog implements the metric registry interface for a Datadog provider.
package datadog

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dogstatsd "github.com/DataDog/datadog-go/statsd"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

const defaultMetricPrefix = "limiter."
const defaultPollFrequency = time.Second * 5

type metricSampleListener struct {
	client     *dogstatsd.Client
	id         string
	metricType uint8
}

// AddSample will add a sample metric to the listener
func (l *metricSampleListener) AddSample(value float64, tags ...string) {
	switch l.metricType {
	case 0: // distribution
		l.client.Distribution(l.id, value, tags, 1.0)
	case 1: // timing
		l.client.TimeInMilliseconds(l.id, value, tags, 1.0)
	case 2: // count
		l.client.Count(l.id, int64(value), tags, 1.0)
	default:
		// unsupported
	}
}

type metricPoller struct {
	supplier core.MetricSupplier
	id       string
	tags     []string
}

func (p *metricPoller) poll() (string, float64, []string, bool) {
	val, ok := p.supplier()
	return p.id, val, p.tags, ok
}

// MetricRegistry will implements a MetricRegistry for sending metrics to Datadog via dogstatsd.
type MetricRegistry struct {
	client              *dogstatsd.Client
	prefix              string
	pollFrequency       time.Duration
	registeredGauges    map[string]*metricPoller
	registeredListeners map[string]*metricSampleListener

	mu sync.Mutex
	wg sync.WaitGroup

	started bool
	stopper chan bool
}

// NewMetricRegistry will create a new Datadog MetricRegistry.
// This registry reports metrics to datadog using the datadog dogstatsd forwarding.
func NewMetricRegistry(addr string, prefix string, pollFrequency time.Duration) (*MetricRegistry, error) {
	if prefix == "" {
		prefix = defaultMetricPrefix
	}
	if !strings.HasSuffix(prefix, ".") {
		prefix = prefix + "."
	}

	if pollFrequency == 0 {
		pollFrequency = defaultPollFrequency
	}

	client, err := dogstatsd.New(addr)
	if err != nil {
		return nil, err
	}
	return &MetricRegistry{
		client:              client,
		prefix:              prefix,
		pollFrequency:       pollFrequency,
		stopper:             make(chan bool, 1),
		registeredGauges:    make(map[string]*metricPoller, 0),
		registeredListeners: make(map[string]*metricSampleListener, 0),
	}, nil
}

// NewMetricRegistryWithClient will create a new Datadog MetricRegistry with the provided client instead.
// This registry reports metrics to datadog using the datadog dogstatsd forwarding.
func NewMetricRegistryWithClient(
	client *dogstatsd.Client,
	prefix string,
	pollFrequency time.Duration,
) (*MetricRegistry, error) {
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if !strings.HasSuffix(prefix, ".") {
		prefix = prefix + "."
	}

	if pollFrequency == 0 {
		pollFrequency = defaultPollFrequency
	}

	return &MetricRegistry{
		client:              client,
		prefix:              prefix,
		pollFrequency:       pollFrequency,
		stopper:             make(chan bool, 1),
		registeredGauges:    make(map[string]*metricPoller, 0),
		registeredListeners: make(map[string]*metricSampleListener, 0),
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
				metricSuffix, value, tags, ok := g.poll()
				if ok {
					r.client.Gauge(r.prefix+metricSuffix, value, tags, 1.0)
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
		client:     r.client,
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
		client:     r.client,
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
		client:     r.client,
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

	r.registeredGauges[ID] = &metricPoller{
		supplier: supplier,
		id:       ID,
		tags:     tags,
	}
}
