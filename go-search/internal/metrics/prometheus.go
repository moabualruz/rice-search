package metrics

import (
	"fmt"
	"sort"
	"strings"
)

// PrometheusFormat exports all metrics in Prometheus text exposition format.
// See: https://prometheus.io/docs/instrumenting/exposition_formats/
func (m *Metrics) PrometheusFormat() string {
	var sb strings.Builder

	// Search metrics
	writeCounter(&sb, m.SearchRequests)
	writeHistogram(&sb, m.SearchLatency)
	writeHistogram(&sb, m.SearchResults)
	writeCounterVec(&sb, m.SearchErrors)
	writeHistogramVec(&sb, m.SearchStageDuration)

	// Index metrics
	writeCounter(&sb, m.IndexedDocuments)
	writeCounter(&sb, m.IndexedChunks)
	writeHistogram(&sb, m.IndexLatency)
	writeCounterVec(&sb, m.IndexErrors)

	// Model metrics
	writeCounter(&sb, m.EmbedRequests)
	writeHistogram(&sb, m.EmbedLatency)
	writeHistogram(&sb, m.EmbedBatchSize)
	writeCounter(&sb, m.RerankRequests)
	writeHistogram(&sb, m.RerankLatency)
	writeCounter(&sb, m.QueryUnderstandRequests)
	writeHistogram(&sb, m.QueryUnderstandLatency)
	writeCounter(&sb, m.SparseEncodeRequests)
	writeHistogram(&sb, m.SparseEncodeLatency)

	// Connection metrics
	writeGauge(&sb, m.ActiveConnections)
	writeCounter(&sb, m.ConnectionsTotal)
	writeCounterVec(&sb, m.ConnectionErrors)

	// Store metrics
	writeGauge(&sb, m.StoresTotal)
	writeGaugeVec(&sb, m.DocumentsTotal)
	writeGaugeVec(&sb, m.ChunksTotal)

	// System metrics
	writeGauge(&sb, m.GoroutineCount)
	writeGauge(&sb, m.MemoryUsage)
	writeCounter(&sb, m.Uptime)

	// Bus metrics
	writeCounterVec(&sb, m.BusEventsPublished)
	writeHistogramVec(&sb, m.BusEventLatency)
	writeCounterVec(&sb, m.BusErrors)

	// HTTP metrics
	writeCounterVec(&sb, m.HTTPRequests)
	writeHistogramVec(&sb, m.HTTPDuration)
	writeGauge(&sb, m.HTTPRequestsInFlight)
	writeHistogramVec(&sb, m.HTTPRequestSize)

	return sb.String()
}

// writeCounter writes a counter in Prometheus format.
func writeCounter(sb *strings.Builder, c *Counter) {
	sb.WriteString("# HELP ")
	sb.WriteString(c.Name())
	sb.WriteString(" ")
	sb.WriteString(c.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(c.Name())
	sb.WriteString(" counter\n")

	sb.WriteString(c.Name())
	writeLabels(sb, c.Labels())
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%d", c.Value()))
	sb.WriteString("\n")
}

// writeGauge writes a gauge in Prometheus format.
func writeGauge(sb *strings.Builder, g *Gauge) {
	sb.WriteString("# HELP ")
	sb.WriteString(g.Name())
	sb.WriteString(" ")
	sb.WriteString(g.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(g.Name())
	sb.WriteString(" gauge\n")

	sb.WriteString(g.Name())
	writeLabels(sb, g.Labels())
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%.0f", g.Value()))
	sb.WriteString("\n")
}

// writeHistogram writes a histogram in Prometheus format.
func writeHistogram(sb *strings.Builder, h *Histogram) {
	sb.WriteString("# HELP ")
	sb.WriteString(h.Name())
	sb.WriteString(" ")
	sb.WriteString(h.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(h.Name())
	sb.WriteString(" histogram\n")

	buckets := h.Buckets()
	counts := h.BucketCounts()
	labels := h.Labels()

	// Write bucket counts
	for i, bucket := range buckets {
		sb.WriteString(h.Name())
		sb.WriteString("_bucket")
		// Merge labels with le bucket label
		bucketLabels := make(map[string]string, len(labels)+1)
		for k, v := range labels {
			bucketLabels[k] = v
		}
		bucketLabels["le"] = fmt.Sprintf("%.3f", bucket)
		writeLabels(sb, bucketLabels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d", counts[i]))
		sb.WriteString("\n")
	}

	// Write +Inf bucket
	sb.WriteString(h.Name())
	sb.WriteString("_bucket")
	infLabels := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		infLabels[k] = v
	}
	infLabels["le"] = "+Inf"
	writeLabels(sb, infLabels)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%d", counts[len(counts)-1]))
	sb.WriteString("\n")

	// Write sum
	sb.WriteString(h.Name())
	sb.WriteString("_sum")
	writeLabels(sb, labels)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%.6f", h.Sum()))
	sb.WriteString("\n")

	// Write count
	sb.WriteString(h.Name())
	sb.WriteString("_count")
	writeLabels(sb, labels)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%d", h.Count()))
	sb.WriteString("\n")
}

// writeCounterVec writes a counter vector in Prometheus format.
func writeCounterVec(sb *strings.Builder, cv *CounterVec) {
	counters := cv.GetAll()
	if len(counters) == 0 {
		return
	}

	sb.WriteString("# HELP ")
	sb.WriteString(cv.Name())
	sb.WriteString(" ")
	sb.WriteString(cv.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(cv.Name())
	sb.WriteString(" counter\n")

	for _, c := range counters {
		sb.WriteString(c.Name())
		writeLabels(sb, c.Labels())
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d", c.Value()))
		sb.WriteString("\n")
	}
}

// writeGaugeVec writes a gauge vector in Prometheus format.
func writeGaugeVec(sb *strings.Builder, gv *GaugeVec) {
	gauges := gv.GetAll()
	if len(gauges) == 0 {
		return
	}

	sb.WriteString("# HELP ")
	sb.WriteString(gv.Name())
	sb.WriteString(" ")
	sb.WriteString(gv.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(gv.Name())
	sb.WriteString(" gauge\n")

	for _, g := range gauges {
		sb.WriteString(g.Name())
		writeLabels(sb, g.Labels())
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%.0f", g.Value()))
		sb.WriteString("\n")
	}
}

// writeHistogramVec writes a histogram vector in Prometheus format.
func writeHistogramVec(sb *strings.Builder, hv *HistogramVec) {
	histograms := hv.GetAll()
	if len(histograms) == 0 {
		return
	}

	sb.WriteString("# HELP ")
	sb.WriteString(hv.Name())
	sb.WriteString(" ")
	sb.WriteString(hv.Help())
	sb.WriteString("\n")

	sb.WriteString("# TYPE ")
	sb.WriteString(hv.Name())
	sb.WriteString(" histogram\n")

	for _, h := range histograms {
		buckets := h.Buckets()
		counts := h.BucketCounts()
		labels := h.Labels()

		// Write bucket counts
		for i, bucket := range buckets {
			sb.WriteString(h.Name())
			sb.WriteString("_bucket")
			// Merge labels with le bucket label
			bucketLabels := make(map[string]string, len(labels)+1)
			for k, v := range labels {
				bucketLabels[k] = v
			}
			bucketLabels["le"] = fmt.Sprintf("%.3f", bucket)
			writeLabels(sb, bucketLabels)
			sb.WriteString(" ")
			sb.WriteString(fmt.Sprintf("%d", counts[i]))
			sb.WriteString("\n")
		}

		// Write +Inf bucket
		sb.WriteString(h.Name())
		sb.WriteString("_bucket")
		infLabels := make(map[string]string, len(labels)+1)
		for k, v := range labels {
			infLabels[k] = v
		}
		infLabels["le"] = "+Inf"
		writeLabels(sb, infLabels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d", counts[len(counts)-1]))
		sb.WriteString("\n")

		// Write sum
		sb.WriteString(h.Name())
		sb.WriteString("_sum")
		writeLabels(sb, labels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%.6f", h.Sum()))
		sb.WriteString("\n")

		// Write count
		sb.WriteString(h.Name())
		sb.WriteString("_count")
		writeLabels(sb, labels)
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%d", h.Count()))
		sb.WriteString("\n")
	}
}

// writeLabels writes labels in Prometheus format {key="value",key2="value2"}.
func writeLabels(sb *strings.Builder, labels map[string]string) {
	if len(labels) == 0 {
		return
	}

	// Sort keys for stable output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sb.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(k)
		sb.WriteString("=\"")
		sb.WriteString(escapeString(labels[k]))
		sb.WriteString("\"")
	}
	sb.WriteString("}")
}

// escapeString escapes special characters in label values.
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
