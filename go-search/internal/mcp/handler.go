package mcp

import (
	"context"
	"encoding/json"

	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/store"
)

type Handler struct {
	search *search.Service
	index  *index.Pipeline
	stores *store.Service

	// Cached tool definitions
	tools []Tool
}

type HandlerConfig struct {
	SearchService *search.Service
	IndexService  *index.Pipeline
	StoreService  *store.Service
}

func NewHandler(cfg HandlerConfig) *Handler {
	h := &Handler{
		search: cfg.SearchService,
		index:  cfg.IndexService,
		stores: cfg.StoreService,
	}
	h.tools = h.defineTools()
	return h
}

func (h *Handler) Handle(ctx context.Context, req *Request) *Response {
	// Simple panic recovery for handler safety
	defer func() {
		if r := recover(); r != nil {
			// In production integration handling
		}
	}()

	switch req.Method {
	// Lifecycle
	case "initialize":
		return h.handleInitialize(req)
	case "initialized":
		return nil // Notification, no response

	// Tools
	case "tools/list":
		return h.handleToolsList(req)
	case "tools/call":
		return h.handleToolsCall(ctx, req)

	// Resources - Stubbed for now
	case "resources/list":
		return h.handleResourcesList(req)
	case "resources/read":
		return h.handleResourcesRead(ctx, req)
	case "resources/templates/list":
		return h.handleResourceTemplatesList(req)

	// Prompts - Stubbed for now
	case "prompts/list":
		return h.handlePromptsList(req)
	case "prompts/get":
		return h.handlePromptsGet(ctx, req)

	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrMethodNotFound, Message: "Method not found"},
		}
	}
}

func (h *Handler) handleInitialize(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05", // Spec version
			"serverInfo": map[string]string{
				"name":    "rice-search",
				"version": "1.0.0",
			},
			"capabilities": ServerCapabilities{
				Tools:     &ToolsCapability{},
				Resources: &ResourcesCapability{Subscribe: false},
				Prompts:   &PromptsCapability{},
			},
		},
	}
}

func (h *Handler) handleToolsList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": h.tools},
	}
}

func (h *Handler) handleToolsCall(ctx context.Context, req *Request) *Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrInvalidParams, Message: err.Error()},
		}
	}

	result, err := h.callTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrInternal, Message: err.Error()},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result},
			},
		},
	}
}

// Stubs for other capabilities
func (h *Handler) handleResourcesList(req *Request) *Response {
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"resources": []Resource{}}}
}

func (h *Handler) handleResourcesRead(ctx context.Context, req *Request) *Response {
	return &Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: ErrMethodNotFound, Message: "Not implemented"}}
}

func (h *Handler) handleResourceTemplatesList(req *Request) *Response {
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"resourceTemplates": []ResourceTemplate{}}}
}

func (h *Handler) handlePromptsList(req *Request) *Response {
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"prompts": []Prompt{}}}
}

func (h *Handler) handlePromptsGet(ctx context.Context, req *Request) *Response {
	return &Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: ErrMethodNotFound, Message: "Not implemented"}}
}
