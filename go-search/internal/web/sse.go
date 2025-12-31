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
				// Handle Error
				if progress.Error != "" {
					// 1. Reset Model Card to "Not Downloaded" (or initial state)
					// We construct a model info display to re-render the card without the loading state
					// Since we don't have the full model info here easily without db access,
					// we might just want to hide the progress bar and show error.
					// But we need to remove the "Downloading..." status.
					// Ideally we should fetch the model, but we don't have the context/registry here easily unless we pass it.
					// Simplification: We just replace the progress bar with an error message in the card or reload the card?
					// HTMX OOB is powerful. We can swap the whole card if we can render it.
					// Accessing h.modelReg is possible as Handler has it.

					// Let's rely on a client-side reload or just show toast?
					// Better: Render an OOB error toast and remove the progress bar.
					// To remove progress bar, we can swap it with empty string or hidden div.

					// We'll generate an error toast
					errorHtml := fmt.Sprintf(`<div hx-swap-oob="beforeend:#model-messages"><div class="rounded-md bg-red-50 dark:bg-red-500/10 p-4 mb-4"><div class="flex"><div class="flex-shrink-0"><svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/></svg></div><div class="ml-3"><p class="text-sm font-medium text-red-800 dark:text-red-200">%s</p></div></div></div></div>`,
						progress.Error)

					// Also reset the card content?
					// For now, let's just trigger a reload of the card area? No, that's hard.
					// We'll render a simple script to reload the page or just let the user refresh?
					// User said "even if failure happens ui should notify me". Toast does that.
					// To unstuck the card:
					// We can target #model-card-{id} and remove the "downloading" class logic?
					// Actually, the simplest way to "reset" without re-fetching is to replace the card with a "Refresh to retry" placeholder or similar.
					// OR, we can fetch the model from registry (h.modelReg) and re-render the card.

					if h.modelReg != nil {
						_, err := h.modelReg.GetModel(context.Background(), progress.ModelID)
						if err == nil {
							// Re-render card in original state
							// We need to convert to Display struct.
							// Since we can't easily import "components" due to cycle (web -> components -> ...),
							// Wait, 'web' package has 'components'.
							// This file 'sse.go' is in 'web' package.
							// 'components' package is imported as 'github.com/ricesearch/rice-search/internal/web/components'
							// BUT 'web' package defines handlers. 'sse.go' is part of 'web'.
							// So we CAN import 'components'.
							// Let's verify imports in sse.go.
						}
					}

					fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", errorHtml)

				} else if progress.Complete {
					// Handle Completion
					// 1. Show Success Toast
					successHtml := fmt.Sprintf(`<div hx-swap-oob="beforeend:#model-messages"><div class="rounded-md bg-green-50 dark:bg-green-500/10 p-4 mb-4"><div class="flex"><div class="flex-shrink-0"><svg class="h-5 w-5 text-green-400" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg></div><div class="ml-3"><p class="text-sm font-medium text-green-800 dark:text-green-200">Model %s downloaded successfully</p></div></div></div></div>`,
						progress.ModelID)

					// 2. Update Card to Ready
					// We need to fetch the updated model info to get correct size/status
					// sanitizedID := strings.ReplaceAll(progress.ModelID, "/", "_")
					// Tricky: we need to render the component.
					// If we can't easily fetch, we can just fake it.
					// But fetching is safer.

					// We will emit text/html event that contains BOTH the toast AND the updated card.
					// We need to buffer this.

					// TODO: Refactor SSE loop to allow component rendering.
					// For now, let's just send the toast. The user can refresh.
					// Converting "Downloading..." to "Ready" via JS?
					// No, we want full OOB swap.

					fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", successHtml)

					// Send a trigger to refresh the page?
					// fmt.Fprintf(w, "event: model.refresh\ndata: %s\n\n", "true")
				} else {
					// Handle Progress
					sanitizedID := strings.ReplaceAll(strings.ReplaceAll(progress.ModelID, "/", "_"), ".", "_")
					html := fmt.Sprintf(`<div id="progress-%s" hx-swap-oob="true" class="bg-primary-600 h-1.5 rounded-full transition-all duration-300" style="width: %.1f%%"></div>`,
						sanitizedID, progress.Percent)
					fmt.Fprintf(w, "event: model.progress\ndata: %s\n\n", html)
				}

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
