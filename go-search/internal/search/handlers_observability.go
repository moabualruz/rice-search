package search

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ricesearch/rice-search/internal/observability"
)

func (h *Handler) handleObservabilityExport(w http.ResponseWriter, r *http.Request) {
	if h.observability == nil {
		writeError(w, http.StatusServiceUnavailable, "observability service not available")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "jsonl"
	}

	store := r.URL.Query().Get("store")

	// Parse date range
	var from, to time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, _ = time.Parse("2006-01-02", toStr)
	}
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		days, _ := strconv.Atoi(daysStr)
		to = time.Now()
		from = to.AddDate(0, 0, -days)
	}

	// Default: last 7 days
	if from.IsZero() {
		to = time.Now()
		from = to.AddDate(0, 0, -7)
	}

	// Get queries
	queries, err := h.observability.GetQueriesInRange(r.Context(), store, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Set headers based on format
	filename := fmt.Sprintf("queries_%s_%s.%s",
		from.Format("20060102"),
		to.Format("20060102"),
		format)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	switch format {
	case "jsonl":
		h.exportJSONL(w, queries)
	case "csv":
		h.exportCSV(w, queries)
	default:
		writeError(w, http.StatusBadRequest, "Invalid format. Use 'jsonl' or 'csv'")
	}
}

func (h *Handler) exportJSONL(w http.ResponseWriter, queries []observability.QueryLogEntry) {
	w.Header().Set("Content-Type", "application/x-ndjson")

	encoder := json.NewEncoder(w)
	for _, q := range queries {
		encoder.Encode(q)
	}
}

func (h *Handler) exportCSV(w http.ResponseWriter, queries []observability.QueryLogEntry) {
	w.Header().Set("Content-Type", "text/csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	writer.Write([]string{
		"timestamp", "store", "query", "intent", "strategy",
		"difficulty", "confidence", "results", "latency_ms",
		"rerank_enabled", "rerank_latency_ms",
	})

	// Data
	for _, q := range queries {
		writer.Write([]string{
			q.Timestamp.Format(time.RFC3339),
			q.Store,
			q.Query,
			q.Intent,
			q.Strategy,
			q.Difficulty,
			fmt.Sprintf("%.2f", q.Confidence),
			strconv.Itoa(q.ResultCount),
			strconv.Itoa(int(q.LatencyMs)),
			strconv.FormatBool(q.RerankEnabled),
			strconv.Itoa(int(q.RerankLatencyMs)),
		})
	}
}
