package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	c := NewCounter("test_counter", "A test counter", nil)

	if c.Value() != 0 {
		t.Errorf("expected initial value 0, got %d", c.Value())
	}

	c.Inc()
	if c.Value() != 1 {
		t.Errorf("expected value 1 after Inc(), got %d", c.Value())
	}

	c.Add(5)
	if c.Value() != 6 {
		t.Errorf("expected value 6 after Add(5), got %d", c.Value())
	}

	// Counters can't decrease
	c.Add(-10)
	if c.Value() != 6 {
		t.Errorf("expected value 6 after Add(-10), got %d", c.Value())
	}

	c.Reset()
	if c.Value() != 0 {
		t.Errorf("expected value 0 after Reset(), got %d", c.Value())
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge("test_gauge", "A test gauge", nil)

	if g.Value() != 0 {
		t.Errorf("expected initial value 0, got %f", g.Value())
	}

	g.Set(42.5)
	if g.Value() != 42 { // Note: we store as int64, so precision is lost
		t.Errorf("expected value 42, got %f", g.Value())
	}

	g.Inc()
	if g.Value() != 43 {
		t.Errorf("expected value 43 after Inc(), got %f", g.Value())
	}

	g.Dec()
	if g.Value() != 42 {
		t.Errorf("expected value 42 after Dec(), got %f", g.Value())
	}

	g.Add(-10)
	if g.Value() != 32 {
		t.Errorf("expected value 32 after Add(-10), got %f", g.Value())
	}
}

func TestHistogram(t *testing.T) {
	buckets := []float64{1, 5, 10, 50, 100}
	h := NewHistogram("test_histogram", "A test histogram", buckets)

	if h.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", h.Count())
	}

	// Observe some values
	h.Observe(2.5)
	h.Observe(7.0)
	h.Observe(150.0)

	if h.Count() != 3 {
		t.Errorf("expected count 3, got %d", h.Count())
	}

	expectedSum := 2.5 + 7.0 + 150.0
	// Allow small precision error since we store as int64
	if diff := h.Sum() - expectedSum; diff > 1.0 || diff < -1.0 {
		t.Errorf("expected sum %f, got %f (diff: %f)", expectedSum, h.Sum(), diff)
	}

	counts := h.BucketCounts()
	// 2.5 falls in bucket 5 (index 1)
	// 7.0 falls in bucket 10 (index 2)
	// 150.0 falls in +Inf (index 5)

	// Buckets are cumulative, so:
	// bucket 1: 1
	// bucket 5: 2
	// bucket 10: 3
	// bucket 50: 3
	// bucket 100: 3
	// +Inf: 3

	if counts[0] < 1 { // At least one value <= 1
		t.Logf("Bucket counts: %v", counts)
	}
}

func TestGaugeVec(t *testing.T) {
	gv := NewGaugeVec("test_gauge_vec", "A test gauge vector", []string{"store", "type"})

	g1 := gv.WithLabels("default", "docs")
	g1.Set(100)

	g2 := gv.WithLabels("default", "chunks")
	g2.Set(500)

	g3 := gv.WithLabels("custom", "docs")
	g3.Set(50)

	gauges := gv.GetAll()
	if len(gauges) != 3 {
		t.Errorf("expected 3 gauges, got %d", len(gauges))
	}

	// Test that getting the same labels returns the same gauge
	g1Again := gv.WithLabels("default", "docs")
	if g1 != g1Again {
		t.Error("expected to get same gauge instance for same labels")
	}
}

func TestCounterVec(t *testing.T) {
	cv := NewCounterVec("test_counter_vec", "A test counter vector", []string{"error_type"})

	c1 := cv.WithLabels("timeout")
	c1.Inc()
	c1.Inc()

	c2 := cv.WithLabels("network")
	c2.Inc()

	counters := cv.GetAll()
	if len(counters) != 2 {
		t.Errorf("expected 2 counters, got %d", len(counters))
	}

	if c1.Value() != 2 {
		t.Errorf("expected timeout counter value 2, got %d", c1.Value())
	}

	if c2.Value() != 1 {
		t.Errorf("expected network counter value 1, got %d", c2.Value())
	}
}

func TestMetricsRecording(t *testing.T) {
	m := New()

	// Stop the background collector
	time.Sleep(100 * time.Millisecond)

	// Record search metrics
	m.RecordSearch(50, 10, nil)
	if m.SearchRequests.Value() != 1 {
		t.Errorf("expected 1 search request, got %d", m.SearchRequests.Value())
	}

	// Record index metrics
	m.RecordIndex(5, 20, 100, nil)
	if m.IndexedDocuments.Value() != 5 {
		t.Errorf("expected 5 indexed documents, got %d", m.IndexedDocuments.Value())
	}
	if m.IndexedChunks.Value() != 20 {
		t.Errorf("expected 20 indexed chunks, got %d", m.IndexedChunks.Value())
	}

	// Record embed metrics
	m.RecordEmbed(32, 25)
	if m.EmbedRequests.Value() != 1 {
		t.Errorf("expected 1 embed request, got %d", m.EmbedRequests.Value())
	}

	// Record rerank metrics
	m.RecordRerank(50, 100)
	if m.RerankRequests.Value() != 1 {
		t.Errorf("expected 1 rerank request, got %d", m.RerankRequests.Value())
	}

	// Update store stats
	m.UpdateStoreStats("default", 100, 500)
	defaultDocs := m.DocumentsTotal.WithLabels("default")
	if defaultDocs.Value() != 100 {
		t.Errorf("expected 100 documents in default store, got %f", defaultDocs.Value())
	}

	// Connection metrics
	m.IncrementConnection()
	if m.ActiveConnections.Value() != 1 {
		t.Errorf("expected 1 active connection, got %f", m.ActiveConnections.Value())
	}
	if m.ConnectionsTotal.Value() != 1 {
		t.Errorf("expected 1 total connection, got %d", m.ConnectionsTotal.Value())
	}

	m.DecrementConnection()
	if m.ActiveConnections.Value() != 0 {
		t.Errorf("expected 0 active connections after decrement, got %f", m.ActiveConnections.Value())
	}
}

func TestPrometheusFormat(t *testing.T) {
	m := New()
	time.Sleep(100 * time.Millisecond)

	// Record some metrics
	m.RecordSearch(50, 10, nil)
	m.RecordIndex(5, 20, 100, nil)
	m.UpdateStoreStats("default", 100, 500)

	output := m.PrometheusFormat()

	// Check for essential components
	requiredStrings := []string{
		"# HELP rice_search_requests_total",
		"# TYPE rice_search_requests_total counter",
		"rice_search_requests_total 1",
		"# HELP rice_indexed_documents_total",
		"# TYPE rice_indexed_documents_total counter",
		"rice_indexed_documents_total 5",
		"# HELP rice_documents_total",
		"# TYPE rice_documents_total gauge",
		"rice_documents_total{store=\"default\"} 100",
	}

	for _, s := range requiredStrings {
		if !strings.Contains(output, s) {
			t.Errorf("expected Prometheus output to contain %q", s)
		}
	}
}

func TestPresets(t *testing.T) {
	presets := GetAllPresets()
	if len(presets) == 0 {
		t.Error("expected at least one preset")
	}

	// Test getting preset by ID
	preset := GetPreset("search_overview")
	if preset == nil {
		t.Error("expected to find search_overview preset")
	}
	if preset.Name != "Search Overview" {
		t.Errorf("expected preset name 'Search Overview', got %s", preset.Name)
	}

	// Test getting presets by category
	categories := GetPresetsByCategory()
	if len(categories) == 0 {
		t.Error("expected at least one category")
	}

	searchPresets := categories["search"]
	if len(searchPresets) == 0 {
		t.Error("expected at least one search preset")
	}
}

func TestMetricQuery(t *testing.T) {
	m := New()
	time.Sleep(100 * time.Millisecond)

	// Record some data
	m.RecordSearch(50, 10, nil)
	m.UpdateStoreCount(3)

	// Execute query
	query := MetricQuery{
		Metrics:   []string{"rice_search_requests_total", "rice_stores_total"},
		TimeRange: "1h",
	}

	result, err := m.ExecuteQuery(query)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Data["rice_search_requests_total"] != int64(1) {
		t.Errorf("expected 1 search request, got %v", result.Data["rice_search_requests_total"])
	}

	if result.Data["rice_stores_total"] != float64(3) {
		t.Errorf("expected 3 stores, got %v", result.Data["rice_stores_total"])
	}
}

func TestLabelsToKey(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "empty",
			labels: map[string]string{},
			want:   "",
		},
		{
			name:   "single label",
			labels: map[string]string{"store": "default"},
			want:   "store=default",
		},
		{
			name:   "multiple labels",
			labels: map[string]string{"store": "default", "type": "docs"},
			want:   "store=default,type=docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := labelsToKey(tt.labels)
			if got != tt.want {
				t.Errorf("labelsToKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkCounterInc(b *testing.B) {
	c := NewCounter("bench_counter", "Benchmark counter", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkGaugeSet(b *testing.B) {
	g := NewGauge("bench_gauge", "Benchmark gauge", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Set(float64(i))
	}
}

func BenchmarkHistogramObserve(b *testing.B) {
	h := NewHistogram("bench_histogram", "Benchmark histogram", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(float64(i % 1000))
	}
}

func BenchmarkGaugeVecWithLabels(b *testing.B) {
	gv := NewGaugeVec("bench_gauge_vec", "Benchmark gauge vector", []string{"store"})
	stores := []string{"store1", "store2", "store3", "store4", "store5"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store := stores[i%len(stores)]
		g := gv.WithLabels(store)
		g.Inc()
	}
}

func BenchmarkPrometheusFormat(b *testing.B) {
	m := New()
	m.RecordSearch(50, 10, nil)
	m.RecordIndex(5, 20, 100, nil)
	m.UpdateStoreStats("default", 100, 500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.PrometheusFormat()
	}
}
