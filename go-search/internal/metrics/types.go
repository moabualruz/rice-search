// Package metrics provides Prometheus-compatible metrics for Rice Search.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter represents a monotonically increasing counter.
type Counter struct {
	name   string
	help   string
	value  int64
	labels map[string]string
	mu     sync.RWMutex
}

// NewCounter creates a new counter.
func NewCounter(name, help string, labels map[string]string) *Counter {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Counter{
		name:   name,
		help:   help,
		labels: labels,
	}
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds the given value to the counter.
func (c *Counter) Add(delta int64) {
	if delta < 0 {
		return // Counters can't decrease
	}
	atomic.AddInt64(&c.value, delta)
}

// Value returns the current counter value.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Reset resets the counter to 0.
func (c *Counter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

// Name returns the metric name.
func (c *Counter) Name() string {
	return c.name
}

// Help returns the metric help text.
func (c *Counter) Help() string {
	return c.help
}

// Labels returns the metric labels.
func (c *Counter) Labels() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]string, len(c.labels))
	for k, v := range c.labels {
		result[k] = v
	}
	return result
}

// Gauge represents a gauge metric that can go up and down.
type Gauge struct {
	name   string
	help   string
	value  int64 // Using int64 for atomic ops, actual value is float64
	labels map[string]string
	mu     sync.RWMutex
}

// NewGauge creates a new gauge.
func NewGauge(name, help string, labels map[string]string) *Gauge {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Gauge{
		name:   name,
		help:   help,
		labels: labels,
	}
}

// Set sets the gauge to the given value.
func (g *Gauge) Set(value float64) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Add adds the given value to the gauge.
func (g *Gauge) Add(delta float64) {
	atomic.AddInt64(&g.value, int64(delta))
}

// Value returns the current gauge value.
func (g *Gauge) Value() float64 {
	return float64(atomic.LoadInt64(&g.value))
}

// Name returns the metric name.
func (g *Gauge) Name() string {
	return g.name
}

// Help returns the metric help text.
func (g *Gauge) Help() string {
	return g.help
}

// Labels returns the metric labels.
func (g *Gauge) Labels() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make(map[string]string, len(g.labels))
	for k, v := range g.labels {
		result[k] = v
	}
	return result
}

// Histogram represents a histogram with buckets.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	counts  []int64
	sum     int64 // Using int64 for atomic ops
	count   int64
	labels  map[string]string
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram with the given buckets.
func NewHistogram(name, help string, buckets []float64) *Histogram {
	if buckets == nil || len(buckets) == 0 {
		// Default buckets in milliseconds
		buckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	}
	// Ensure buckets are sorted
	sort.Float64s(buckets)

	return &Histogram{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1), // +1 for +Inf
	}
}

// Observe adds a single observation.
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update sum and count
	atomic.AddInt64(&h.sum, int64(value))
	atomic.AddInt64(&h.count, 1)

	// Find bucket
	bucketIdx := len(h.buckets) // Start with +Inf bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIdx = i
			break
		}
	}

	// Increment bucket and all higher buckets (cumulative)
	for i := bucketIdx; i < len(h.counts); i++ {
		atomic.AddInt64(&h.counts[i], 1)
	}
}

// Count returns the total count of observations.
func (h *Histogram) Count() int64 {
	return atomic.LoadInt64(&h.count)
}

// Sum returns the sum of all observed values.
func (h *Histogram) Sum() float64 {
	return float64(atomic.LoadInt64(&h.sum))
}

// Buckets returns the bucket upper bounds.
func (h *Histogram) Buckets() []float64 {
	result := make([]float64, len(h.buckets))
	copy(result, h.buckets)
	return result
}

// BucketCounts returns the count for each bucket.
func (h *Histogram) BucketCounts() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]int64, len(h.counts))
	for i := range h.counts {
		result[i] = atomic.LoadInt64(&h.counts[i])
	}
	return result
}

// Name returns the metric name.
func (h *Histogram) Name() string {
	return h.name
}

// Help returns the metric help text.
func (h *Histogram) Help() string {
	return h.help
}

// Labels returns the metric labels.
func (h *Histogram) Labels() map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.labels == nil {
		return make(map[string]string)
	}
	result := make(map[string]string, len(h.labels))
	for k, v := range h.labels {
		result[k] = v
	}
	return result
}

// GaugeVec represents a gauge with labels.
type GaugeVec struct {
	name       string
	help       string
	labelNames []string
	gauges     map[string]*Gauge
	mu         sync.RWMutex
}

// NewGaugeVec creates a new gauge vector.
func NewGaugeVec(name, help string, labelNames []string) *GaugeVec {
	return &GaugeVec{
		name:       name,
		help:       help,
		labelNames: labelNames,
		gauges:     make(map[string]*Gauge),
	}
}

// WithLabels returns a gauge with the given label values.
func (gv *GaugeVec) WithLabels(labelValues ...string) *Gauge {
	if len(labelValues) != len(gv.labelNames) {
		panic(fmt.Sprintf("expected %d label values, got %d", len(gv.labelNames), len(labelValues)))
	}

	// Create label map
	labels := make(map[string]string, len(gv.labelNames))
	for i, name := range gv.labelNames {
		labels[name] = labelValues[i]
	}

	// Create stable key from sorted labels
	key := labelsToKey(labels)

	gv.mu.RLock()
	gauge, exists := gv.gauges[key]
	gv.mu.RUnlock()

	if exists {
		return gauge
	}

	// Create new gauge
	gv.mu.Lock()
	defer gv.mu.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists := gv.gauges[key]; exists {
		return gauge
	}

	gauge = NewGauge(gv.name, gv.help, labels)
	gv.gauges[key] = gauge
	return gauge
}

// GetAll returns all gauges in the vector.
func (gv *GaugeVec) GetAll() []*Gauge {
	gv.mu.RLock()
	defer gv.mu.RUnlock()

	result := make([]*Gauge, 0, len(gv.gauges))
	for _, g := range gv.gauges {
		result = append(result, g)
	}
	return result
}

// Name returns the metric name.
func (gv *GaugeVec) Name() string {
	return gv.name
}

// Help returns the metric help text.
func (gv *GaugeVec) Help() string {
	return gv.help
}

// CounterVec represents a counter with labels.
type CounterVec struct {
	name       string
	help       string
	labelNames []string
	counters   map[string]*Counter
	mu         sync.RWMutex
}

// NewCounterVec creates a new counter vector.
func NewCounterVec(name, help string, labelNames []string) *CounterVec {
	return &CounterVec{
		name:       name,
		help:       help,
		labelNames: labelNames,
		counters:   make(map[string]*Counter),
	}
}

// WithLabels returns a counter with the given label values.
func (cv *CounterVec) WithLabels(labelValues ...string) *Counter {
	if len(labelValues) != len(cv.labelNames) {
		panic(fmt.Sprintf("expected %d label values, got %d", len(cv.labelNames), len(labelValues)))
	}

	// Create label map
	labels := make(map[string]string, len(cv.labelNames))
	for i, name := range cv.labelNames {
		labels[name] = labelValues[i]
	}

	// Create stable key from sorted labels
	key := labelsToKey(labels)

	cv.mu.RLock()
	counter, exists := cv.counters[key]
	cv.mu.RUnlock()

	if exists {
		return counter
	}

	// Create new counter
	cv.mu.Lock()
	defer cv.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := cv.counters[key]; exists {
		return counter
	}

	counter = NewCounter(cv.name, cv.help, labels)
	cv.counters[key] = counter
	return counter
}

// GetAll returns all counters in the vector.
func (cv *CounterVec) GetAll() []*Counter {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	result := make([]*Counter, 0, len(cv.counters))
	for _, c := range cv.counters {
		result = append(result, c)
	}
	return result
}

// Name returns the metric name.
func (cv *CounterVec) Name() string {
	return cv.name
}

// Help returns the metric help text.
func (cv *CounterVec) Help() string {
	return cv.help
}

// HistogramVec represents a histogram with labels.
type HistogramVec struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64
	histograms map[string]*Histogram
	mu         sync.RWMutex
}

// NewHistogramVec creates a new histogram vector.
func NewHistogramVec(name, help string, labelNames []string, buckets []float64) *HistogramVec {
	return &HistogramVec{
		name:       name,
		help:       help,
		labelNames: labelNames,
		buckets:    buckets,
		histograms: make(map[string]*Histogram),
	}
}

// WithLabels returns a histogram with the given label values.
func (hv *HistogramVec) WithLabels(labelValues ...string) *Histogram {
	if len(labelValues) != len(hv.labelNames) {
		panic(fmt.Sprintf("expected %d label values, got %d", len(hv.labelNames), len(labelValues)))
	}

	// Create label map
	labels := make(map[string]string, len(hv.labelNames))
	for i, name := range hv.labelNames {
		labels[name] = labelValues[i]
	}

	// Create stable key from sorted labels
	key := labelsToKey(labels)

	hv.mu.RLock()
	histogram, exists := hv.histograms[key]
	hv.mu.RUnlock()

	if exists {
		return histogram
	}

	// Create new histogram
	hv.mu.Lock()
	defer hv.mu.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists := hv.histograms[key]; exists {
		return histogram
	}

	histogram = NewHistogram(hv.name, hv.help, hv.buckets)
	// Store labels for Prometheus export
	histogram.mu.Lock()
	histogram.labels = labels
	histogram.mu.Unlock()

	hv.histograms[key] = histogram
	return histogram
}

// GetAll returns all histograms in the vector.
func (hv *HistogramVec) GetAll() []*Histogram {
	hv.mu.RLock()
	defer hv.mu.RUnlock()

	result := make([]*Histogram, 0, len(hv.histograms))
	for _, h := range hv.histograms {
		result = append(result, h)
	}
	return result
}

// Name returns the metric name.
func (hv *HistogramVec) Name() string {
	return hv.name
}

// Help returns the metric help text.
func (hv *HistogramVec) Help() string {
	return hv.help
}

// labelsToKey creates a stable key from label map.
func labelsToKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	// Sort keys for stable ordering
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build key
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(labels[k])
	}
	return sb.String()
}
