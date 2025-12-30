# Testing Strategy

## Overview

Testing pyramid: Unit → Integration → E2E.

---

## Test Categories

| Category | Purpose | Speed | Dependencies |
|----------|---------|-------|--------------|
| Unit | Test isolated logic | Fast (<1s) | None |
| Integration | Test component interactions | Medium (1-30s) | Mocked services |
| E2E | Test full system | Slow (30s+) | Real services |

---

## Unit Tests

### What to Unit Test

| Component | Test |
|-----------|------|
| Chunker | Chunk boundaries, overlap, sizes |
| RRF Fusion | Score calculation, ranking |
| Validators | Input validation rules |
| Config | Parsing, defaults, validation |
| Event types | Serialization, deserialization |

### Example: Chunker Test

```go
func TestChunker_ChunkCode(t *testing.T) {
    tests := []struct {
        name     string
        content  string
        language string
        wantLen  int
    }{
        {
            name:     "small file single chunk",
            content:  "func main() {\n    fmt.Println(\"hello\")\n}",
            language: "go",
            wantLen:  1,
        },
        {
            name:     "large file multiple chunks",
            content:  largeGoFile,
            language: "go",
            wantLen:  5,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            chunker := NewChunker(ChunkConfig{
                TargetSize: 512,
                Overlap:    64,
            })
            
            chunks, err := chunker.Chunk(tt.content, tt.language)
            
            assert.NoError(t, err)
            assert.Len(t, chunks, tt.wantLen)
        })
    }
}
```

### Example: RRF Test

```go
func TestRRFFusion(t *testing.T) {
    sparse := []Result{
        {ID: "a", Score: 10.0, Rank: 1},
        {ID: "b", Score: 8.0, Rank: 2},
        {ID: "c", Score: 5.0, Rank: 3},
    }
    
    dense := []Result{
        {ID: "b", Score: 0.9, Rank: 1},
        {ID: "a", Score: 0.7, Rank: 2},
        {ID: "d", Score: 0.6, Rank: 3},
    }
    
    fused := RRFFusion(sparse, dense, 60)
    
    // b should rank first (rank 1 in dense, rank 2 in sparse)
    assert.Equal(t, "b", fused[0].ID)
}
```

### Running Unit Tests

```bash
go test ./internal/... -short -v
```

---

## Integration Tests

### What to Integration Test

| Integration | Test |
|-------------|------|
| ML + ONNX | Model loading, inference accuracy |
| Search + Qdrant | Query execution, filtering |
| API + Bus | Event publishing, handling |
| Index pipeline | Full chunking → embedding → storage |

### Test Infrastructure

> **Note**: Integration tests use Docker Compose for Qdrant instead of testcontainers.
> Start Qdrant with: `docker-compose -f deployments/docker-compose.dev.yml up -d`

```go
// testutil/qdrant.go (simplified approach)

func GetQdrantURL(t *testing.T) string {
    url := os.Getenv("QDRANT_URL")
    if url == "" {
        url = "http://localhost:6333"
    }
    
    // Verify Qdrant is accessible
    resp, err := http.Get(url + "/readyz")
    if err != nil {
        t.Skipf("Qdrant not available at %s: %v", url, err)
    }
    resp.Body.Close()
    
    return url
}
```

### Example: Search Integration Test

```go
func TestSearch_HybridSearch(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Start Qdrant
    qdrant := testutil.StartQdrant(t)
    
    // Create service with real Qdrant
    service := search.NewService(search.Config{
        QdrantURL: qdrant.URL(),
    })
    
    // Index test documents
    err := service.Index(ctx, IndexRequest{
        Store: "test",
        Documents: []Document{
            {Path: "auth.go", Content: "func Authenticate() {}"},
            {Path: "user.go", Content: "type User struct {}"},
        },
    })
    require.NoError(t, err)
    
    // Search
    resp, err := service.Search(ctx, SearchRequest{
        Store: "test",
        Query: "authentication",
        TopK:  10,
    })
    
    require.NoError(t, err)
    assert.NotEmpty(t, resp.Results)
    assert.Equal(t, "auth.go", resp.Results[0].Path)
}
```

### Running Integration Tests

```bash
# All integration tests
go test ./internal/... -v

# Specific package
go test ./internal/search/... -v
```

---

## E2E Tests

> ⚠️ **NOT IMPLEMENTED**: E2E tests are documented for future implementation. Currently only unit and integration tests are available.

### What to E2E Test (Planned)

| Scenario | Test | Status |
|----------|------|--------|
| Full search flow | HTTP → Event → Search → Response | ❌ Not implemented |
| Full index flow | HTTP → Chunk → Embed → Store | ❌ Not implemented |
| CLI operations | Command line → API → Result | ❌ Not implemented |
| Failure recovery | Kill service, restart, verify | ❌ Not implemented |

### Example: E2E Search Test

```go
func TestE2E_SearchFlow(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }
    
    // Start full system
    env := testutil.StartFullEnvironment(t)
    
    // Index via HTTP
    resp, err := http.Post(
        env.APIURL+"/v1/stores/test/index",
        "application/json",
        strings.NewReader(`{
            "documents": [
                {"path": "main.go", "content": "package main\nfunc main() {}"}
            ]
        }`),
    )
    require.NoError(t, err)
    require.Equal(t, 200, resp.StatusCode)
    
    // Wait for indexing
    time.Sleep(2 * time.Second)
    
    // Search via HTTP
    resp, err = http.Post(
        env.APIURL+"/v1/stores/test/search",
        "application/json",
        strings.NewReader(`{"query": "main function"}`),
    )
    require.NoError(t, err)
    require.Equal(t, 200, resp.StatusCode)
    
    // Verify results
    var result SearchResponse
    json.NewDecoder(resp.Body).Decode(&result)
    assert.NotEmpty(t, result.Results)
}
```

### Running E2E Tests

```bash
# Requires Docker
go test ./e2e/... -v -timeout 10m
```

---

## Test Fixtures

### Sample Data

```
testdata/
├── files/
│   ├── small.go        # < 100 lines
│   ├── medium.go       # 100-500 lines
│   ├── large.go        # > 500 lines
│   └── multilang/
│       ├── auth.ts
│       ├── auth.py
│       └── auth.rs
├── queries/
│   └── benchmark.txt   # 100 sample queries
└── expected/
    └── search_results.json
```

### Golden Files

```go
func TestSearch_GoldenFile(t *testing.T) {
    result := search("authentication handler")
    
    golden := filepath.Join("testdata", "expected", t.Name()+".json")
    
    if *update {
        // Update golden file
        writeJSON(golden, result)
    }
    
    expected := readJSON(golden)
    assert.Equal(t, expected, result)
}
```

---

## Mocking

### ML Service Mock

```go
type MockMLService struct {
    mock.Mock
}

func (m *MockMLService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    args := m.Called(ctx, texts)
    return args.Get(0).([][]float32), args.Error(1)
}

// Usage
func TestSearch_WithMockedML(t *testing.T) {
    mlMock := &MockMLService{}
    mlMock.On("Embed", mock.Anything, mock.Anything).Return(
        [][]float32{{0.1, 0.2, 0.3}},
        nil,
    )
    
    service := search.NewService(search.Config{
        ML: mlMock,
    })
    
    // Test...
    
    mlMock.AssertExpectations(t)
}
```

### Event Bus Mock

```go
type MockBus struct {
    published []Event
    mu        sync.Mutex
}

func (m *MockBus) Publish(ctx context.Context, topic string, event any) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.published = append(m.published, Event{Topic: topic, Data: event})
    return nil
}

func (m *MockBus) GetPublished(topic string) []Event {
    m.mu.Lock()
    defer m.mu.Unlock()
    var result []Event
    for _, e := range m.published {
        if e.Topic == topic {
            result = append(result, e)
        }
    }
    return result
}
```

---

## Test Coverage

### Coverage Targets

| Package | Target |
|---------|--------|
| `internal/search` | 80% |
| `internal/ml` | 70% |
| `internal/index` | 80% |
| `internal/api` | 70% |
| `internal/bus` | 90% |

### Running with Coverage

```bash
# Generate coverage
go test ./internal/... -coverprofile=coverage.out

# View report
go tool cover -html=coverage.out

# Check threshold
go tool cover -func=coverage.out | grep total
```

---

## CI/CD Pipeline

### GitHub Actions

```yaml
name: Test

on: [push, pull_request]

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./internal/... -short -race -cover

  integration:
    runs-on: ubuntu-latest
    services:
      qdrant:
        image: qdrant/qdrant:v1.12.4
        ports:
          - 6333:6333
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test ./internal/... -race
        env:
          QDRANT_URL: http://localhost:6333

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: docker-compose up -d
      - run: go test ./e2e/... -timeout 10m
      - run: docker-compose down
```

---

## Test Commands

```bash
# All unit tests
make test-unit

# All integration tests
make test-integration

# All E2E tests
make test-e2e

# All tests with coverage
make test-coverage

# Benchmark tests
make benchmark
```
