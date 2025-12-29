package models

// DefaultModels returns the default model registry.
var DefaultModels = []ModelInfo{
	{
		ID:          "jinaai/jina-code-embeddings-1.5b",
		Type:        ModelTypeEmbed,
		DisplayName: "Jina Code Embeddings 1.5B",
		Description: "Code-optimized dense embeddings with 1536 dimensions, optimized for code search",
		OutputDim:   1536,
		MaxTokens:   8192,
		Downloaded:  false,
		IsDefault:   true,
		GPUEnabled:  true,
		Size:        1610612736, // ~1.5GB
		DownloadURL: "https://huggingface.co/jinaai/jina-code-embeddings-1.5b",
	},
	{
		ID:          "jinaai/jina-reranker-v2-base-multilingual",
		Type:        ModelTypeRerank,
		DisplayName: "Jina Reranker v2",
		Description: "Fast multilingual reranking model with code awareness",
		MaxTokens:   512,
		Downloaded:  false,
		IsDefault:   true,
		GPUEnabled:  true,
		Size:        838860800, // ~800MB
		DownloadURL: "https://huggingface.co/jinaai/jina-reranker-v2-base-multilingual",
	},
	{
		ID:          "Salesforce/codet5p-220m",
		Type:        ModelTypeQueryUnderstand,
		DisplayName: "CodeT5+ 220M",
		Description: "Query understanding and intent classification for code search",
		MaxTokens:   512,
		Downloaded:  false,
		IsDefault:   true,
		GPUEnabled:  false,     // Disabled by default, falls back to Option C (heuristics)
		Size:        230686720, // ~220MB
		DownloadURL: "https://huggingface.co/Salesforce/codet5p-220m",
	},
}

// DefaultTypeConfigs returns the default model type configurations.
var DefaultTypeConfigs = []ModelTypeConfig{
	{
		Type:         ModelTypeEmbed,
		DefaultModel: "jinaai/jina-code-embeddings-1.5b",
		GPUEnabled:   true,
	},
	{
		Type:         ModelTypeRerank,
		DefaultModel: "jinaai/jina-reranker-v2-base-multilingual",
		GPUEnabled:   true,
	},
	{
		Type:         ModelTypeQueryUnderstand,
		DefaultModel: "Salesforce/codet5p-220m",
		GPUEnabled:   false,
		Fallback:     "heuristic", // Falls back to heuristic-based query understanding
	},
}

// DefaultMappers returns the default model mappers.
var DefaultMappers = []ModelMapper{
	{
		ID:             "jina-code-embeddings-1.5b-mapper",
		Name:           "Jina Code Embeddings Mapper",
		ModelID:        "jinaai/jina-code-embeddings-1.5b",
		Type:           ModelTypeEmbed,
		PromptTemplate: "",
		InputMapping: map[string]string{
			"text": "text",
		},
		OutputMapping: map[string]string{
			"embedding": "embedding",
		},
	},
	{
		ID:             "jina-reranker-v2-mapper",
		Name:           "Jina Reranker v2 Mapper",
		ModelID:        "jinaai/jina-reranker-v2-base-multilingual",
		Type:           ModelTypeRerank,
		PromptTemplate: "",
		InputMapping: map[string]string{
			"query":    "query",
			"document": "document",
		},
		OutputMapping: map[string]string{
			"score": "score",
		},
	},
	{
		ID:             "codet5p-mapper",
		Name:           "CodeT5+ Query Understanding Mapper",
		ModelID:        "Salesforce/codet5p-220m",
		Type:           ModelTypeQueryUnderstand,
		PromptTemplate: "",
		InputMapping: map[string]string{
			"query": "text",
		},
		OutputMapping: map[string]string{
			"intent":     "intent",
			"difficulty": "difficulty",
			"confidence": "confidence",
		},
	},
}
