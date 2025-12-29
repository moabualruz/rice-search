package ml

import (
	"context"
	"encoding/json"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// EventHandler handles ML events from the event bus.
type EventHandler struct {
	svc Service
	bus bus.Bus
	log *logger.Logger
}

// NewEventHandler creates a new ML event handler.
func NewEventHandler(svc Service, b bus.Bus, log *logger.Logger) *EventHandler {
	return &EventHandler{
		svc: svc,
		bus: b,
		log: log,
	}
}

// Register registers all ML event handlers.
func (h *EventHandler) Register(ctx context.Context) error {
	handlers := map[string]bus.Handler{
		bus.TopicEmbedRequest:  h.handleEmbed,
		bus.TopicSparseRequest: h.handleSparse,
		bus.TopicRerankRequest: h.handleRerank,
	}

	for topic, handler := range handlers {
		if err := h.bus.Subscribe(ctx, topic, handler); err != nil {
			return errors.Wrap(errors.CodeInternal, "failed to subscribe to "+topic, err)
		}
	}

	return nil
}

// EmbedRequest is the payload for embed requests.
type EmbedRequest struct {
	Texts []string `json:"texts"`
}

// EmbedResponse is the payload for embed responses.
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

func (h *EventHandler) handleEmbed(ctx context.Context, event bus.Event) error {
	// Parse request
	var req EmbedRequest
	if err := unmarshalPayload(event.Payload, &req); err != nil {
		return h.respondError(event, bus.TopicEmbedResponse, err)
	}

	// Execute
	embeddings, err := h.svc.Embed(ctx, req.Texts)
	if err != nil {
		return h.respondError(event, bus.TopicEmbedResponse, err)
	}

	// Respond
	return h.respond(event, bus.TopicEmbedResponse, EmbedResponse{
		Embeddings: embeddings,
	})
}

// SparseRequest is the payload for sparse encoding requests.
type SparseRequest struct {
	Texts []string `json:"texts"`
}

// SparseResponse is the payload for sparse encoding responses.
type SparseResponse struct {
	Vectors []SparseVector `json:"vectors"`
	Error   string         `json:"error,omitempty"`
}

func (h *EventHandler) handleSparse(ctx context.Context, event bus.Event) error {
	// Parse request
	var req SparseRequest
	if err := unmarshalPayload(event.Payload, &req); err != nil {
		return h.respondError(event, bus.TopicSparseResponse, err)
	}

	// Execute
	vectors, err := h.svc.SparseEncode(ctx, req.Texts)
	if err != nil {
		return h.respondError(event, bus.TopicSparseResponse, err)
	}

	// Respond
	return h.respond(event, bus.TopicSparseResponse, SparseResponse{
		Vectors: vectors,
	})
}

// RerankRequest is the payload for rerank requests.
type RerankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopK      int      `json:"top_k"`
}

// RerankResponse is the payload for rerank responses.
type RerankResponse struct {
	Results []RankedResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

func (h *EventHandler) handleRerank(ctx context.Context, event bus.Event) error {
	// Parse request
	var req RerankRequest
	if err := unmarshalPayload(event.Payload, &req); err != nil {
		return h.respondError(event, bus.TopicRerankResponse, err)
	}

	// Execute
	results, err := h.svc.Rerank(ctx, req.Query, req.Documents, req.TopK)
	if err != nil {
		return h.respondError(event, bus.TopicRerankResponse, err)
	}

	// Respond
	return h.respond(event, bus.TopicRerankResponse, RerankResponse{
		Results: results,
	})
}

func (h *EventHandler) respond(event bus.Event, topic string, payload any) error {
	// For memory bus, use Respond method if available
	if memBus, ok := h.bus.(*bus.MemoryBus); ok {
		return memBus.Respond(event.CorrelationID, bus.Event{
			ID:            event.ID + "-response",
			Type:          topic,
			Source:        "ml",
			CorrelationID: event.CorrelationID,
			Payload:       payload,
		})
	}

	// Otherwise publish to response topic
	return h.bus.Publish(context.Background(), topic, bus.Event{
		ID:            event.ID + "-response",
		Type:          topic,
		Source:        "ml",
		CorrelationID: event.CorrelationID,
		Payload:       payload,
	})
}

func (h *EventHandler) respondError(event bus.Event, topic string, err error) error {
	h.log.Error("ML handler error", "topic", topic, "error", err)

	errResp := struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	}

	return h.respond(event, topic, errResp)
}

func unmarshalPayload(payload any, target any) error {
	// If payload is already the right type, copy it
	if data, ok := payload.([]byte); ok {
		return json.Unmarshal(data, target)
	}

	// If payload is a map or struct, marshal/unmarshal
	data, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(errors.CodeValidation, "invalid payload", err)
	}

	return json.Unmarshal(data, target)
}
