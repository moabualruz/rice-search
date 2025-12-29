# Models Package

The `models` package provides ML model management and configuration mapping for Rice Search.

## Overview

This package handles:
- **Model Registry** - Catalog of available ML models with metadata
- **Model Mappers** - Input/output mapping configurations for models
- **Type Configurations** - Default model settings per type
- **Storage** - YAML-based persistence for all configurations

## Architecture

```
models/
├── types.go           # Core data structures
├── defaults.go        # Default models and configurations
├── registry.go        # Model registry service
├── mapper_service.go  # Mapper CRUD service
├── storage.go         # YAML storage implementation
└── *_test.go          # Test files
```

## Model Types

Three model types are supported:

| Type | Purpose | Required Fields |
|------|---------|----------------|
| `embed` | Dense embeddings | `output_dim` |
| `rerank` | Result reranking | - |
| `query_understand` | Query intent classification | - |

## Core Types

### ModelInfo

Describes a registered ML model:

```go
type ModelInfo struct {
    ID           string    // "jinaai/jina-code-embeddings-1.5b"
    Type         ModelType // embed, rerank, query_understand
    DisplayName  string    // "Jina Code Embeddings 1.5B"
    Description  string
    OutputDim    int       // For embedding models only
    MaxTokens    int       // Max sequence length
    Downloaded   bool      // Installation status
    IsDefault    bool      // Default for this type
    GPUEnabled   bool      // GPU acceleration enabled
    Size         int64     // Model size in bytes
    DownloadURL  string    // HuggingFace URL
}
```

### ModelMapper

Defines input/output mapping for a model:

```go
type ModelMapper struct {
    ID             string            // "jina-code-embeddings-1.5b-mapper"
    Name           string            // "Jina Code Embeddings Mapper"
    ModelID        string            // "jinaai/jina-code-embeddings-1.5b"
    Type           ModelType
    PromptTemplate string            // Optional prompt template
    InputMapping   map[string]string // {"text": "text"}
    OutputMapping  map[string]string // {"embedding": "embedding"}
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### ModelTypeConfig

Configuration for a model type:

```go
type ModelTypeConfig struct {
    Type         ModelType // embed, rerank, query_understand
    DefaultModel string    // "jinaai/jina-code-embeddings-1.5b"
    GPUEnabled   bool      // GPU enabled for this type
    Fallback     string    // Optional fallback (e.g., "heuristic")
}
```

## Default Models

### Embedding Model
- **ID**: `jinaai/jina-code-embeddings-1.5b`
- **Dimensions**: 1536
- **Max Tokens**: 8192
- **GPU**: Enabled by default
- **Size**: ~1.5GB

### Reranking Model
- **ID**: `jinaai/jina-reranker-v2-base-multilingual`
- **Max Tokens**: 512
- **GPU**: Enabled by default
- **Size**: ~800MB

### Query Understanding Model
- **ID**: `Salesforce/codet5p-220m`
- **Max Tokens**: 512
- **GPU**: **Disabled by default** (falls back to heuristics)
- **Size**: ~220MB

## Registry Service

The `Registry` manages the model catalog.

### Creating a Registry

```go
cfg := RegistryConfig{
    StoragePath:  "./data/models",
    ModelsDir:    "./models",
    LoadDefaults: true,
}

log := logger.New(logger.Config{Level: "info"})
registry, err := NewRegistry(cfg, log)
```

### Registry Operations

```go
// List all models
models, err := registry.ListModels(ctx, ModelTypeEmbed)

// Get a specific model
model, err := registry.GetModel(ctx, "jinaai/jina-code-embeddings-1.5b")

// Set default for a type
err := registry.SetDefaultModel(ctx, ModelTypeEmbed, "new-model-id")

// Toggle GPU
err := registry.ToggleGPU(ctx, "model-id", true)

// Get default for type
model, err := registry.GetDefaultForType(ctx, ModelTypeEmbed)

// Download model (returns progress channel)
progressChan, err := registry.DownloadModel(ctx, "model-id")
for progress := range progressChan {
    fmt.Printf("Downloaded: %.2f%%\n", progress.Percent)
}

// Register new model
newModel := &ModelInfo{...}
err := registry.RegisterModel(ctx, newModel)

// Delete model
err := registry.DeleteModel(ctx, "model-id")
```

## Mapper Service

The `MapperService` manages model mappers.

### Creating a Mapper Service

```go
storage := NewFileStorage("./data/models")
mapperService, err := NewMapperService(storage, registry, log)
```

### Mapper Operations

```go
// List all mappers
mappers, err := mapperService.ListMappers(ctx)

// Get specific mapper
mapper, err := mapperService.GetMapper(ctx, "mapper-id")

// Get mapper for a model
mapper, err := mapperService.GetMapperForModel(ctx, "model-id")

// Create mapper
newMapper := &ModelMapper{...}
err := mapperService.CreateMapper(ctx, newMapper)

// Update mapper
mapper.InputMapping["new_field"] = "field"
err := mapperService.UpdateMapper(ctx, mapper)

// Delete mapper
err := mapperService.DeleteMapper(ctx, "mapper-id")

// Auto-generate mapper from model
mapper, err := mapperService.GenerateMapper(ctx, "model-id")
```

## Storage

Two storage implementations are provided:

### MemoryStorage

In-memory storage for testing:

```go
storage := NewMemoryStorage()
```

### FileStorage

YAML-based file storage:

```go
storage := NewFileStorage("./data/models")
```

**Directory Structure:**
```
./data/models/
├── models.yaml           # All models in one file
├── type_configs.yaml     # All type configs in one file
└── mappers/              # One file per mapper
    ├── mapper1.yaml
    ├── mapper2.yaml
    └── mapper3.yaml
```

### Storage Interface

```go
type Storage interface {
    // Model operations
    SaveModel(model *ModelInfo) error
    LoadModel(id string) (*ModelInfo, error)
    LoadAllModels() ([]*ModelInfo, error)
    DeleteModel(id string) error
    ModelExists(id string) bool

    // Mapper operations
    SaveMapper(mapper *ModelMapper) error
    LoadMapper(id string) (*ModelMapper, error)
    LoadAllMappers() ([]*ModelMapper, error)
    DeleteMapper(id string) error
    MapperExists(id string) bool

    // Type config operations
    SaveTypeConfig(cfg *ModelTypeConfig) error
    LoadTypeConfig(modelType ModelType) (*ModelTypeConfig, error)
    LoadAllTypeConfigs() ([]*ModelTypeConfig, error)
    DeleteTypeConfig(modelType ModelType) error
}
```

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "github.com/ricesearch/rice-search/internal/models"
    "github.com/ricesearch/rice-search/internal/pkg/logger"
)

func main() {
    ctx := context.Background()
    log := logger.New(logger.Config{Level: "info"})

    // Create registry
    registry, err := models.NewRegistry(models.RegistryConfig{
        StoragePath:  "./data/models",
        ModelsDir:    "./models",
        LoadDefaults: true,
    }, log)
    if err != nil {
        panic(err)
    }

    // Create mapper service
    storage := models.NewFileStorage("./data/models")
    mapperService, err := models.NewMapperService(storage, registry, log)
    if err != nil {
        panic(err)
    }

    // Use services...
}
```

### Switching Default Models

```go
// List available embedding models
models, _ := registry.ListModels(ctx, models.ModelTypeEmbed)
for _, m := range models {
    fmt.Printf("%s: %s\n", m.ID, m.DisplayName)
}

// Set new default
err := registry.SetDefaultModel(ctx, models.ModelTypeEmbed, "new-model-id")

// Verify
defaultModel, _ := registry.GetDefaultForType(ctx, models.ModelTypeEmbed)
fmt.Printf("Default: %s\n", defaultModel.ID)
```

### Custom Model Registration

```go
customModel := &models.ModelInfo{
    ID:          "custom/my-embeddings",
    Type:        models.ModelTypeEmbed,
    DisplayName: "My Custom Embeddings",
    Description: "Custom trained embeddings",
    OutputDim:   768,
    MaxTokens:   512,
    Downloaded:  true,
    IsDefault:   false,
    GPUEnabled:  false,
    Size:        500_000_000, // 500MB
    DownloadURL: "https://example.com/model",
}

if err := registry.RegisterModel(ctx, customModel); err != nil {
    panic(err)
}

// Auto-generate mapper
mapper, err := mapperService.GenerateMapper(ctx, "custom/my-embeddings")
```

### Downloading Models

```go
progressChan, err := registry.DownloadModel(ctx, "jinaai/jina-code-embeddings-1.5b")
if err != nil {
    panic(err)
}

for progress := range progressChan {
    if progress.Complete {
        fmt.Println("Download complete!")
    } else if progress.Error != "" {
        fmt.Printf("Error: %s\n", progress.Error)
    } else {
        fmt.Printf("Progress: %.2f%% (%d/%d bytes)\n", 
            progress.Percent, progress.Downloaded, progress.Total)
    }
}
```

## Testing

Run tests:

```bash
cd go-search
go test ./internal/models/...
```

Run with coverage:

```bash
go test -cover ./internal/models/...
```

Run with verbose output:

```bash
go test -v ./internal/models/...
```

## Configuration Files

### models.yaml

```yaml
- id: jinaai/jina-code-embeddings-1.5b
  type: embed
  display_name: Jina Code Embeddings 1.5B
  description: Code-optimized dense embeddings
  output_dim: 1536
  max_tokens: 8192
  downloaded: false
  is_default: true
  gpu_enabled: true
  size: 1610612736
  download_url: https://huggingface.co/jinaai/jina-code-embeddings-1.5b
```

### type_configs.yaml

```yaml
- type: embed
  default_model: jinaai/jina-code-embeddings-1.5b
  gpu_enabled: true
- type: rerank
  default_model: jinaai/jina-reranker-v2-base-multilingual
  gpu_enabled: true
- type: query_understand
  default_model: Salesforce/codet5p-220m
  gpu_enabled: false
  fallback: heuristic
```

### mappers/jina-code-embeddings-1.5b-mapper.yaml

```yaml
id: jina-code-embeddings-1.5b-mapper
name: Jina Code Embeddings Mapper
model_id: jinaai/jina-code-embeddings-1.5b
type: embed
prompt_template: ""
input_mapping:
  text: text
output_mapping:
  embedding: embedding
created_at: 2025-12-29T00:00:00Z
updated_at: 2025-12-29T00:00:00Z
```

## Thread Safety

All services are thread-safe:
- `Registry` uses `sync.RWMutex` for concurrent access
- `MapperService` uses `sync.RWMutex` for concurrent access
- `MemoryStorage` uses `sync.RWMutex` for concurrent access
- `FileStorage` uses `sync.RWMutex` for concurrent access

## Error Handling

The package uses the `internal/pkg/errors` package for consistent error handling:

```go
model, err := registry.GetModel(ctx, "invalid-id")
if errors.IsNotFound(err) {
    fmt.Println("Model not found")
}

if errors.IsValidation(err) {
    fmt.Println("Invalid input")
}
```

## Integration

This package integrates with:
- **ML Service** (`internal/ml`) - Uses model registry to load models
- **Config** (`internal/config`) - Reads storage paths from config
- **Logger** (`internal/pkg/logger`) - Structured logging
- **Errors** (`internal/pkg/errors`) - Error handling

## Future Enhancements

Potential improvements:
1. **Real Model Downloads** - Implement actual HuggingFace downloads
2. **Model Versioning** - Track model versions and updates
3. **Model Quantization** - Support quantized model variants
4. **Caching** - Cache model metadata for faster lookups
5. **Webhooks** - Notify on model updates/downloads
6. **Metrics** - Track model usage and performance
