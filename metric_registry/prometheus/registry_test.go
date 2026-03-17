package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gather collects all metrics from the registry and returns a map of
// metric family name → MetricFamily for easy assertions.
func gather(t *testing.T, reg *prometheus.Registry) map[string]*dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	m := make(map[string]*dto.MetricFamily, len(mfs))
	for _, mf := range mfs {
		m[mf.GetName()] = mf
	}
	return m
}

func newTestRegistry() (*MetricRegistry, *prometheus.Registry) {
	promReg := prometheus.NewRegistry()
	r := NewMetricRegistryWithRegisterer("limiter", promReg)
	return r, promReg
}

func TestNewMetricRegistry_DefaultNamespace(t *testing.T) {
	r := NewMetricRegistryWithRegisterer("", prometheus.NewRegistry())
	assert.Equal(t, "limiter", r.namespace)
}

func TestStartStopAreNoOps(t *testing.T) {
	r, _ := newTestRegistry()
	// should not panic or block
	r.Start()
	r.Stop()
}

func TestRegisterDistribution(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	listener := r.RegisterDistribution("inflight")
	require.NotNil(t, listener)

	listener.AddSample(5.0)
	listener.AddSample(10.0)

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_inflight"]
	require.True(t, ok, "expected metric limiter_inflight to be registered")
	require.Len(t, mf.GetMetric(), 1)
	assert.Equal(t, uint64(2), mf.GetMetric()[0].GetSummary().GetSampleCount())
}

func TestRegisterDistributionReusesSameListener(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()

	l1 := r.RegisterDistribution("inflight")
	l2 := r.RegisterDistribution("inflight")
	assert.Same(t, l1.(*metricSampleListener), l2.(*metricSampleListener))
}

func TestRegisterTiming(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	listener := r.RegisterTiming("rtt")
	require.NotNil(t, listener)

	listener.AddSample(100.0) // 100 ms
	listener.AddSample(200.0)

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_rtt_ms"]
	require.True(t, ok, "expected metric limiter_rtt_ms to be registered")
	assert.Equal(t, uint64(2), mf.GetMetric()[0].GetSummary().GetSampleCount())
}

func TestRegisterTimingReusesSameListener(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()

	l1 := r.RegisterTiming("rtt")
	l2 := r.RegisterTiming("rtt")
	assert.Same(t, l1.(*metricSampleListener), l2.(*metricSampleListener))
}

func TestRegisterCount(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	listener := r.RegisterCount("dropped")
	require.NotNil(t, listener)

	listener.AddSample(1.0)
	listener.AddSample(1.0)
	listener.AddSample(1.0)

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_dropped_total"]
	require.True(t, ok, "expected metric limiter_dropped_total to be registered")
	assert.Equal(t, 3.0, mf.GetMetric()[0].GetCounter().GetValue())
}

func TestRegisterCountReusesSameListener(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()

	l1 := r.RegisterCount("dropped")
	l2 := r.RegisterCount("dropped")
	assert.Same(t, l1.(*metricSampleListener), l2.(*metricSampleListener))
}

func TestRegisterGauge(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	currentLimit := 10
	r.RegisterGauge("limit", func() (float64, bool) {
		return float64(currentLimit), true
	})

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_limit"]
	require.True(t, ok, "expected metric limiter_limit to be registered")
	assert.Equal(t, 10.0, mf.GetMetric()[0].GetGauge().GetValue())

	// Supplier is polled on each Gather call — updating the value should be reflected.
	currentLimit = 20
	mfs = gather(t, promReg)
	assert.Equal(t, 20.0, mfs["limiter_limit"].GetMetric()[0].GetGauge().GetValue())
}

func TestRegisterGaugeReusesSameGauge(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()

	calls := 0
	supplier := func() (float64, bool) { calls++; return 1, true }

	r.RegisterGauge("limit", supplier)
	r.RegisterGauge("limit", supplier) // second call is a no-op

	assert.Len(t, r.registeredGauges, 1)
}

func TestRegisterGaugeSupplierReturnsFalse(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	r.RegisterGauge("limit", func() (float64, bool) {
		return 0, false // not ok → should report 0
	})

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_limit"]
	require.True(t, ok)
	assert.Equal(t, 0.0, mf.GetMetric()[0].GetGauge().GetValue())
}

func TestTagsBecomeLabelOnDistribution(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	r.RegisterDistribution("inflight", "partition:a")

	mfs := gather(t, promReg)
	mf, ok := mfs["limiter_inflight"]
	require.True(t, ok)
	require.Len(t, mf.GetMetric(), 1)

	labels := mf.GetMetric()[0].GetLabel()
	require.Len(t, labels, 1)
	assert.Equal(t, "partition", labels[0].GetName())
	assert.Equal(t, "a", labels[0].GetValue())
}

func TestSanitizeName(t *testing.T) {
	assert.Equal(t, "window_min_rtt", sanitizeName("window.min_rtt"))
	assert.Equal(t, "inflight", sanitizeName("inflight"))
	assert.Equal(t, "limit_partition", sanitizeName(".limit.partition"))
}

func TestParseTags(t *testing.T) {
	labels := parseTags([]string{"partition:a", "region:us-east", "malformed"})
	assert.Equal(t, prometheus.Labels{
		"partition": "a",
		"region":    "us-east",
	}, labels)
}

func TestMetricNameWithDots(t *testing.T) {
	t.Parallel()
	r, promReg := newTestRegistry()

	r.RegisterDistribution("window.min_rtt")

	mfs := gather(t, promReg)
	_, ok := mfs["limiter_window_min_rtt"]
	assert.True(t, ok, "dots in ID should be replaced with underscores in metric name")
}
