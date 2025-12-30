package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/models"
)

// =============================================================================
// SSE Events
// =============================================================================

func (h *Handler) handleSSEEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush immediately to establish connection
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Create a channel for events
	eventChan := make(chan bus.Event, 10)
	
	// Subscribe to model progress events
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if h.bus != nil {
		err := h.bus.Subscribe(ctx, bus.TopicModelProgress, func(ctx context.Context, event bus.Event) error {
			select {
			case eventChan <- event:
			default:
				// Drop event if channel full
			}
			return nil
		})
		if err != nil {
			h.log.Error("Failed to subscribe to SSE events", "error", err)
			return
		}
	}

	// Keep alive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send keepalive comment
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case event := <-eventChan:
			// Format event for SSE
			// We expect the payload to be models.DownloadProgress
			progress, ok := event.Payload.(models.DownloadProgress)
			if !ok {
				// Try to cast if it was deserialized as map (unlikely within same process but possible with JSON bus)
				continue
			}

			// Render the progress bar component
			// <div id="progress-{id}" ...>
			sanitizedID := strings.ReplaceAll(progress.ModelID, "/", "_")
			html := fmt.Sprintf(`<div id="progress-%s" class="bg-primary-600 h-1.5 rounded-full transition-all duration-300" style="width: %.1f%%"></div>`,
				sanitizedID, progress.Percent)

			// If complete, we might want to trigger a full refresh or replace the "Downloading..." text
			if progress.Complete {
				// Trigger a reload of the model list or verify status
				fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", html)
				// Also send a special event to reload the card?
				// For now, just filling the bar is good visual feedback.
				// The user can refresh or we can use OOB swap to check status.
			} else {
				fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", html)
			}

			flusher.Flush()
		}
	}
}
