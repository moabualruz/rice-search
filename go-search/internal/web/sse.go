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
		// Subscribe to model progress
		err := h.bus.Subscribe(ctx, bus.TopicModelProgress, func(ctx context.Context, event bus.Event) error {
			select {
			case eventChan <- event:
			default:
			}
			return nil
		})
		if err != nil {
			h.log.Error("Failed to subscribe to model progress events", "error", err)
		}

		// Subscribe to index progress
		err = h.bus.Subscribe(ctx, bus.TopicIndexProgress, func(ctx context.Context, event bus.Event) error {
			select {
			case eventChan <- event:
			default:
			}
			return nil
		})
		if err != nil {
			h.log.Error("Failed to subscribe to index progress events", "error", err)
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

			switch event.Type {
			case bus.TopicModelProgress:
				progress, ok := event.Payload.(models.DownloadProgress)
				if !ok {
					continue
				}
				sanitizedID := strings.ReplaceAll(progress.ModelID, "/", "_")
				html := fmt.Sprintf(`<div id="progress-%s" class="bg-primary-600 h-1.5 rounded-full transition-all duration-300" style="width: %.1f%%"></div>`,
					sanitizedID, progress.Percent)
				fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", html)

			case bus.TopicIndexProgress:
				payload, ok := event.Payload.(map[string]interface{})
				if !ok {
					continue
				}
				percent := 0
				if p, ok := payload["percentage"].(int); ok {
					percent = p
				} else if p, ok := payload["percentage"].(float64); ok {
					percent = int(p)
				}

				// Show status in header
				// If 100%, show brief success then hide, or just hide?
				// For continuous indexing, seeing "Indexing 100%" then "Indexing 0%" might be weird.
				// Let's just show "Indexing: XX%"

				displayClass := "flex"
				if percent >= 100 {
					// Fade out after a moment? SSE can't easily wait.
					// We can send a "hidden" class if we want to hide it.
					// But let's just show 100% for now.
				}

				html := fmt.Sprintf(`<div id="global-status" class="%s items-center gap-2 text-primary-400 font-medium whitespace-nowrap"><span class="animate-pulse">●</span> Indexing: %d%%</div>`,
					displayClass, percent)

				fmt.Fprintf(w, "event: index.progress\ndata: %s\n\n", html)
			}

			flusher.Flush()
		}
	}
}
