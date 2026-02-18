package metrics_test

import (
	"testing"

	"github.com/aalemi-dev/stdlib-lab/metrics"
)

// newAppMetrics returns a Metrics with only the application endpoint active.
func newAppMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()
	empty := ""
	port := ":0"
	return metrics.NewMetrics(metrics.Config{
		ServiceName:               t.Name(),
		SystemMetricsAddress:      &empty,
		ApplicationMetricsAddress: &port,
	})
}

// --- Counter ---

func TestCounter_IncAndAdd_NoLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	c := m.CreateCounter("counter_no_labels", "help", []string{})
	c.Inc()
	c.Add(3)
}

func TestCounter_WithLabelValues_ThenInc(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	c := m.CreateCounter("counter_labels", "help", []string{"method"})
	labeled := c.WithLabelValues("GET")
	labeled.Inc()
	labeled.Add(2)
}

func TestCounter_WithLabelValues_ChainedWithLabelValues(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	c := m.CreateCounter("counter_chained", "help", []string{"method"})
	// Calling WithLabelValues on an already-labeled counter returns self
	labeled := c.WithLabelValues("GET")
	_ = labeled.WithLabelValues("ignored")
}

// --- Gauge ---

func TestGauge_AllMethods_NoLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	g := m.CreateGauge("gauge_no_labels", "help", []string{})
	g.Set(10)
	g.Inc()
	g.Dec()
	g.Add(5)
	g.Sub(2)
	g.SetToCurrentTime()
}

func TestGauge_AllMethods_WithLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	g := m.CreateGauge("gauge_labels", "help", []string{"pool"})
	labeled := g.WithLabelValues("primary")
	labeled.Set(100)
	labeled.Inc()
	labeled.Dec()
	labeled.Add(10)
	labeled.Sub(3)
	labeled.SetToCurrentTime()
}

func TestGauge_WithLabelValues_ChainedWithLabelValues(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	g := m.CreateGauge("gauge_chained", "help", []string{"pool"})
	labeled := g.WithLabelValues("primary")
	// Calling WithLabelValues on an already-labeled gauge returns self
	_ = labeled.WithLabelValues("ignored")
}

// --- Histogram ---

func TestHistogram_Observe_NoLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	h := m.CreateHistogram("hist_no_labels", "help", []string{}, []float64{0.1, 0.5, 1.0})
	h.Observe(0.3)
}

func TestHistogram_Observe_WithLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	h := m.CreateHistogram("hist_labels", "help", []string{"endpoint"}, []float64{0.1, 0.5, 1.0})
	h.WithLabelValues("/api").Observe(0.05)
	h.WithLabelValues("/api").Observe(0.9)
}

// --- Summary ---

func TestSummary_Observe_NoLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	s := m.CreateSummary("summary_no_labels", "help", []string{}, map[float64]float64{0.5: 0.05})
	s.Observe(0.2)
}

func TestSummary_Observe_WithLabels(t *testing.T) {
	t.Parallel()
	m := newAppMetrics(t)
	s := m.CreateSummary("summary_labels", "help", []string{"method"}, map[float64]float64{0.9: 0.01})
	s.WithLabelValues("GET").Observe(0.1)
	s.WithLabelValues("POST").Observe(0.4)
}

// --- Default addresses ---

func TestNewMetrics_DefaultAddresses(t *testing.T) {
	t.Parallel()
	m := metrics.NewMetrics(metrics.Config{ServiceName: "defaults"})
	if m.SystemServer == nil {
		t.Error("expected SystemServer with default address")
	}
	if m.ApplicationServer == nil {
		t.Error("expected ApplicationServer with default address")
	}
	// Close immediately to free ports
	_ = m.SystemServer.Close()
	_ = m.ApplicationServer.Close()
}
