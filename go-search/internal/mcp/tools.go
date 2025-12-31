package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/search"
)

func (h *Handler) defineTools() []Tool {
	return []Tool{
		{
			Name:        "code_search",
			Description: "Search files using natural language. Optimized for code but works for any documents. Returns relevant snippets with file paths and line numbers.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Natural language search query (e.g., 'authentication middleware', 'error handling')",
					},
					"store": {
						Type:        "string",
						Description: "Store name (default: 'default')",
					},
					"top_k": {
						Type:        "number",
						Description: "Maximum number of results (default: 10)",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "index_files",
			Description: "Index files into the search database. Supports code, documentation, configs, and any text files.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"files": {
						Type:        "array",
						Description: "Array of {path, content} objects to index",
					},
					"store": {
						Type:        "string",
						Description: "Store name (default: 'default')",
					},
				},
				Required: []string{"files"},
			},
		},
		{
			Name:        "delete_files",
			Description: "Delete files from the search index.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"paths": {
						Type:        "array",
						Description: "Array of file paths to delete",
					},
					"store": {
						Type:        "string",
						Description: "Store name (default: 'default')",
					},
				},
				Required: []string{"paths"},
			},
		},
		{
			Name:        "list_stores",
			Description: "List all available search stores.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "get_store_stats",
			Description: "Get statistics for a search store.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"store": {
						Type:        "string",
						Description: "Store name (default: 'default')",
					},
				},
			},
		},
	}
}

func (h *Handler) callTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "code_search":
		return h.toolCodeSearch(ctx, args)
	case "index_files":
		return h.toolIndexFiles(ctx, args)
	case "delete_files":
		return h.toolDeleteFiles(ctx, args)
	case "list_stores":
		return h.toolListStores(ctx)
	case "get_store_stats":
		return h.toolGetStoreStats(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (h *Handler) toolCodeSearch(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Store string `json:"store"`
		TopK  int    `json:"top_k"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}

	if params.Store == "" {
		params.Store = "default"
	}
	if params.TopK == 0 {
		params.TopK = 10
	}

	// Call search service
	enableReranking := true
	results, err := h.search.Search(ctx, search.Request{
		Store:           params.Store,
		Query:           params.Query,
		TopK:            params.TopK,
		IncludeContent:  true,
		EnableReranking: &enableReranking, // Good default for AI
	})
	if err != nil {
		return "", err
	}

	// Format results as text
	var output string
	for i, r := range results.Results {
		output += fmt.Sprintf("## %d. %s:%d-%d\n", i+1, r.Path, r.StartLine, r.EndLine)
		output += fmt.Sprintf("Language: %s | Score: %.2f\n", r.Language, r.Score)
		if len(r.Symbols) > 0 {
			output += fmt.Sprintf("Symbols: %v\n", r.Symbols)
		}
		output += fmt.Sprintf("```%s\n%s\n```\n\n", r.Language, r.Content)
	}

	return output, nil
}

func (h *Handler) toolIndexFiles(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Files []struct {
			Path     string `json:"path"`
			Content  string `json:"content"`
			Language string `json:"language"`
		} `json:"files"`
		Store string `json:"store"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}

	if params.Store == "" {
		params.Store = "default"
	}

	// Convert to index request
	docs := make([]*index.Document, len(params.Files))
	for i, f := range params.Files {
		doc := index.NewDocument(f.Path, f.Content)
		if f.Language != "" {
			doc.Language = f.Language
		}
		docs[i] = doc
	}

	// Call index service
	result, err := h.index.Index(ctx, index.IndexRequest{
		Store:     params.Store,
		Documents: docs,
		Force:     false,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Indexed %d files (%d chunks)", result.Indexed, result.ChunksTotal), nil
}

func (h *Handler) toolDeleteFiles(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Paths []string `json:"paths"`
		Store string   `json:"store"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}

	if params.Store == "" {
		params.Store = "default"
	}

	// Call index service
	err := h.index.Delete(ctx, params.Store, params.Paths)
	if err != nil {
		return "", err
	}

	// Delete returns error only, no count in Pipeline.Delete signature from outline
	// Only Sync returns count.
	return fmt.Sprintf("Deleted %d paths", len(params.Paths)), nil
}

func (h *Handler) toolListStores(ctx context.Context) (string, error) {
	stores, err := h.stores.ListStores(ctx)
	if err != nil {
		return "", err
	}

	var output string
	for _, s := range stores {
		output += fmt.Sprintf("- %s: %d files, %d chunks\n",
			s.Name, s.Stats.DocumentCount, s.Stats.ChunkCount)
	}

	return output, nil
}

func (h *Handler) toolGetStoreStats(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Store string `json:"store"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}

	if params.Store == "" {
		params.Store = "default"
	}

	s, err := h.stores.GetStore(ctx, params.Store)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Store: %s\nFiles: %d\nChunks: %d\nSize: %d bytes\nLast Indexed: %s",
		s.Name, s.Stats.DocumentCount, s.Stats.ChunkCount,
		s.Stats.TotalSize, s.Stats.LastIndexed.Format("2006-01-02 15:04:05")), nil
}
