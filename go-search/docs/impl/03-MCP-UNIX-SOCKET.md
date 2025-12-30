# Implementation Plan: MCP via Unix Domain Socket

**Priority:** 🔴 P1 (Critical)  
**Effort:** Medium (2-3 days)  
**Dependencies:** None

---

## Overview

Implement Model Context Protocol (MCP) server that listens on a Unix Domain Socket for near-zero latency communication. AI assistants connect to the socket and send/receive JSON-RPC messages as if using stdio.

## Goals

1. **stdio-like experience** - Line-based JSON-RPC, feels like native MCP
2. **Near-zero latency** - Unix socket is 10-100x faster than TCP/gRPC
3. **Multi-client support** - Single server, multiple AI assistants
4. **Full MCP compatibility** - Tools, resources, prompts as per spec

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     rice-search server                           │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    MCP Socket Listener                    │  │
│  │            ~/.local/run/rice-search/mcp.sock              │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│              ┌───────────────┼───────────────┐                  │
│              ▼               ▼               ▼                  │
│     ┌────────────┐   ┌────────────┐   ┌────────────┐           │
│     │  Client 1  │   │  Client 2  │   │  Client 3  │           │
│     │  (Claude)  │   │ (OpenCode) │   │  (Cursor)  │           │
│     └─────┬──────┘   └─────┬──────┘   └─────┬──────┘           │
│           │                │                │                    │
│           └────────────────┼────────────────┘                   │
│                            ▼                                     │
│     ┌──────────────────────────────────────────────────────┐   │
│     │                   MCP Handler                         │   │
│     │   initialize | tools/list | tools/call | resources   │   │
│     └──────────────────────────────────────────────────────┘   │
│                            │                                     │
│         ┌──────────────────┼──────────────────┐                 │
│         ▼                  ▼                  ▼                  │
│  ┌────────────┐    ┌────────────┐    ┌────────────┐            │
│  │   Search   │    │   Index    │    │   Stores   │            │
│  │   Service  │    │   Service  │    │   Service  │            │
│  └────────────┘    └────────────┘    └────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

## MCP Protocol

### Message Format (JSON-RPC 2.0)

**Request:**
```json
{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}}
```

**Response:**
```json
{"jsonrpc": "2.0", "id": 1, "result": {"tools": [...]}}
```

**Notification (no id):**
```json
{"jsonrpc": "2.0", "method": "notifications/progress", "params": {...}}
```

### Lifecycle

1. Client connects to socket
2. Client sends `initialize` request
3. Server responds with capabilities
4. Client sends `initialized` notification
5. Normal request/response flow
6. Client disconnects (or server shuts down)

## Package Structure

```
internal/
├── mcp/
│   ├── server.go          # Unix socket listener
│   ├── handler.go         # JSON-RPC dispatcher
│   ├── protocol.go        # Message types
│   ├── tools.go           # Tool implementations
│   ├── resources.go       # Resource implementations
│   └── prompts.go         # Prompt templates
```

## Implementation Steps

### Step 1: Define Protocol Types

**File:** `internal/mcp/protocol.go`
```go
package mcp

import (
    "encoding/json"
)

// JSON-RPC message types
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`  // string | number | null
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Result  interface{}     `json:"result,omitempty"`
    Error   *Error          `json:"error,omitempty"`
}

type Error struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// Error codes (JSON-RPC standard + MCP extensions)
const (
    ErrParse          = -32700
    ErrInvalidRequest = -32600
    ErrMethodNotFound = -32601
    ErrInvalidParams  = -32602
    ErrInternal       = -32603
)

// MCP capability types
type ServerCapabilities struct {
    Tools     *ToolsCapability     `json:"tools,omitempty"`
    Resources *ResourcesCapability `json:"resources,omitempty"`
    Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

type ToolsCapability struct{}
type ResourcesCapability struct {
    Subscribe bool `json:"subscribe,omitempty"`
}
type PromptsCapability struct{}

// Tool definitions
type Tool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
    Type       string              `json:"type"`
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required,omitempty"`
}

type Property struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}

// Resource definitions
type Resource struct {
    URI         string `json:"uri"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MimeType    string `json:"mimeType,omitempty"`
}

type ResourceTemplate struct {
    URITemplate string `json:"uriTemplate"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MimeType    string `json:"mimeType,omitempty"`
}

// Prompt definitions
type Prompt struct {
    Name        string           `json:"name"`
    Description string           `json:"description,omitempty"`
    Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required,omitempty"`
}
```

### Step 2: Implement Socket Server

**File:** `internal/mcp/server.go`
```go
package mcp

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net"
    "os"
    "path/filepath"
    "sync"
)

type Server struct {
    socketPath string
    handler    *Handler
    listener   net.Listener
    
    // Active connections
    connsMu sync.RWMutex
    conns   map[net.Conn]struct{}
    
    log *slog.Logger
}

type ServerConfig struct {
    SocketPath string  // Default: ~/.local/run/rice-search/mcp.sock
    Handler    *Handler
}

func NewServer(cfg ServerConfig) *Server {
    if cfg.SocketPath == "" {
        home, _ := os.UserHomeDir()
        cfg.SocketPath = filepath.Join(home, ".local", "run", "rice-search", "mcp.sock")
    }
    
    return &Server{
        socketPath: cfg.SocketPath,
        handler:    cfg.Handler,
        conns:      make(map[net.Conn]struct{}),
        log:        slog.Default().With("component", "mcp"),
    }
}

func (s *Server) Start(ctx context.Context) error {
    // Ensure directory exists
    dir := filepath.Dir(s.socketPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create socket dir: %w", err)
    }
    
    // Remove existing socket file
    os.Remove(s.socketPath)
    
    // Create listener
    listener, err := net.Listen("unix", s.socketPath)
    if err != nil {
        return fmt.Errorf("failed to listen on %s: %w", s.socketPath, err)
    }
    s.listener = listener
    
    // Set permissions (readable/writable by owner only)
    os.Chmod(s.socketPath, 0600)
    
    s.log.Info("MCP server listening", "socket", s.socketPath)
    
    // Accept connections
    go s.acceptLoop(ctx)
    
    // Wait for context cancellation
    <-ctx.Done()
    return s.Shutdown()
}

func (s *Server) acceptLoop(ctx context.Context) {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                return
            default:
                s.log.Error("Accept error", "error", err)
                continue
            }
        }
        
        s.connsMu.Lock()
        s.conns[conn] = struct{}{}
        s.connsMu.Unlock()
        
        go s.handleConnection(ctx, conn)
    }
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
    defer func() {
        conn.Close()
        s.connsMu.Lock()
        delete(s.conns, conn)
        s.connsMu.Unlock()
    }()
    
    s.log.Debug("Client connected")
    
    reader := bufio.NewReader(conn)
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        
        // Read line (JSON-RPC message)
        line, err := reader.ReadBytes('\n')
        if err != nil {
            s.log.Debug("Client disconnected", "error", err)
            return
        }
        
        // Parse request
        var req Request
        if err := json.Unmarshal(line, &req); err != nil {
            s.sendError(conn, nil, ErrParse, "Parse error")
            continue
        }
        
        // Handle request
        response := s.handler.Handle(ctx, &req)
        
        // Send response (if not notification)
        if req.ID != nil {
            s.sendResponse(conn, response)
        }
    }
}

func (s *Server) sendResponse(conn net.Conn, resp *Response) {
    data, err := json.Marshal(resp)
    if err != nil {
        s.log.Error("Failed to marshal response", "error", err)
        return
    }
    
    conn.Write(data)
    conn.Write([]byte("\n"))
}

func (s *Server) sendError(conn net.Conn, id interface{}, code int, message string) {
    resp := &Response{
        JSONRPC: "2.0",
        ID:      id,
        Error:   &Error{Code: code, Message: message},
    }
    s.sendResponse(conn, resp)
}

func (s *Server) Shutdown() error {
    s.log.Info("Shutting down MCP server")
    
    // Close listener
    if s.listener != nil {
        s.listener.Close()
    }
    
    // Close all connections
    s.connsMu.Lock()
    for conn := range s.conns {
        conn.Close()
    }
    s.connsMu.Unlock()
    
    // Remove socket file
    os.Remove(s.socketPath)
    
    return nil
}

func (s *Server) SocketPath() string {
    return s.socketPath
}
```

### Step 3: Implement Request Handler

**File:** `internal/mcp/handler.go`
```go
package mcp

import (
    "context"
    "encoding/json"
    
    "github.com/ricesearch/go-search/internal/search"
    "github.com/ricesearch/go-search/internal/index"
    "github.com/ricesearch/go-search/internal/store"
)

type Handler struct {
    search *search.Service
    index  *index.Service
    stores *store.Service
    
    // Cached tool definitions
    tools []Tool
}

type HandlerConfig struct {
    SearchService *search.Service
    IndexService  *index.Service
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
    
    // Resources
    case "resources/list":
        return h.handleResourcesList(req)
    case "resources/read":
        return h.handleResourcesRead(ctx, req)
    case "resources/templates/list":
        return h.handleResourceTemplatesList(req)
    
    // Prompts
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
            "protocolVersion": "2024-11-05",
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
```

### Step 4: Implement Tools

**File:** `internal/mcp/tools.go`
```go
package mcp

import (
    "context"
    "encoding/json"
    "fmt"
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
    results, err := h.search.Search(ctx, search.Request{
        Store:   params.Store,
        Query:   params.Query,
        TopK:    params.TopK,
        Options: search.DefaultOptions(),
    })
    if err != nil {
        return "", err
    }
    
    // Format results as text
    var output string
    for i, r := range results.Results {
        output += fmt.Sprintf("## %d. %s:%d-%d\n", i+1, r.Path, r.StartLine, r.EndLine)
        output += fmt.Sprintf("Language: %s | Score: %.2f\n", r.Language, r.FinalScore)
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
            Path    string `json:"path"`
            Content string `json:"content"`
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
    files := make([]index.File, len(params.Files))
    for i, f := range params.Files {
        files[i] = index.File{Path: f.Path, Content: f.Content}
    }
    
    // Call index service
    result, err := h.index.IndexFiles(ctx, params.Store, files)
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("Indexed %d files (%d chunks)", result.FileCount, result.ChunkCount), nil
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
    count, err := h.index.DeleteFiles(ctx, params.Store, params.Paths)
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("Deleted %d files", count), nil
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
```

### Step 5: Wire into Server

**File:** `cmd/rice-search-server/main.go` (additions)
```go
// Add to server initialization
func main() {
    // ... existing code ...
    
    // Initialize MCP handler
    mcpHandler := mcp.NewHandler(mcp.HandlerConfig{
        SearchService: searchSvc,
        IndexService:  indexSvc,
        StoreService:  storeSvc,
    })
    
    // Start MCP server
    mcpServer := mcp.NewServer(mcp.ServerConfig{
        Handler: mcpHandler,
    })
    
    go func() {
        if err := mcpServer.Start(ctx); err != nil {
            log.Error("MCP server error", "error", err)
        }
    }()
    
    log.Info("MCP server listening", "socket", mcpServer.SocketPath())
    
    // ... rest of main ...
}
```

### Step 6: AI Assistant Configuration

**Claude Desktop (`claude_desktop_config.json`):**
```json
{
  "mcpServers": {
    "rice-search": {
      "command": "socat",
      "args": ["-", "UNIX-CONNECT:~/.local/run/rice-search/mcp.sock"]
    }
  }
}
```

**Alternative using netcat:**
```json
{
  "mcpServers": {
    "rice-search": {
      "command": "nc",
      "args": ["-U", "~/.local/run/rice-search/mcp.sock"]
    }
  }
}
```

**OpenCode (opencode.json):**
```json
{
  "mcp": {
    "servers": {
      "rice-search": {
        "transport": {
          "type": "stdio",
          "command": "socat",
          "args": ["-", "UNIX-CONNECT:~/.local/run/rice-search/mcp.sock"]
        }
      }
    }
  }
}
```

## Windows Support

On Windows, use named pipes instead of Unix sockets:

```go
// internal/mcp/server_windows.go
func (s *Server) Start(ctx context.Context) error {
    pipeName := `\\.\pipe\rice-search-mcp`
    
    listener, err := winio.ListenPipe(pipeName, nil)
    if err != nil {
        return err
    }
    s.listener = listener
    
    // ... rest same as Unix
}
```

**Windows config:**
```json
{
  "mcpServers": {
    "rice-search": {
      "command": "cmd",
      "args": ["/c", "type CON > \\\\.\\pipe\\rice-search-mcp"]
    }
  }
}
```

## Testing

```go
func TestMCPServer_Initialize(t *testing.T) {
    // Start server
    server := mcp.NewServer(mcp.ServerConfig{
        SocketPath: filepath.Join(t.TempDir(), "test.sock"),
        Handler:    mcp.NewHandler(mcp.HandlerConfig{/* mocks */}),
    })
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go server.Start(ctx)
    time.Sleep(100 * time.Millisecond)
    
    // Connect as client
    conn, err := net.Dial("unix", server.SocketPath())
    require.NoError(t, err)
    defer conn.Close()
    
    // Send initialize request
    req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
    conn.Write([]byte(req))
    
    // Read response
    reader := bufio.NewReader(conn)
    line, _ := reader.ReadBytes('\n')
    
    var resp mcp.Response
    json.Unmarshal(line, &resp)
    
    assert.Nil(t, resp.Error)
    assert.NotNil(t, resp.Result)
}
```

## Success Metrics

- [ ] Socket listener starts with server
- [ ] `initialize` handshake works
- [ ] `tools/list` returns 5 tools
- [ ] `tools/call` executes tools correctly
- [ ] Multiple concurrent clients supported
- [ ] Graceful shutdown closes connections
- [ ] Works with Claude Desktop via socat
- [ ] Works on Linux, macOS, Windows

## References

- MCP Specification: https://modelcontextprotocol.io/specification
- Old implementation: `api/src/mcp/mcp.service.ts`
- JSON-RPC 2.0: https://www.jsonrpc.org/specification
