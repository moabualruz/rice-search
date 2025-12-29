# Web UI Dashboard Enhancement - Implementation Status

## Completed Tasks

### 1. Updated layout.templ ✅
- Added `LayoutData` struct with health status, version, and Qdrant URL
- Enhanced navigation with:
  - Dashboard link (new home page)
  - Search, Stores, Files links
  - Admin dropdown (Models, Mappers, Connections)
  - Stats link
- Added status bar showing:
  - Health indicator (green/yellow/red dot)
  - Version display
  - Qdrant dashboard link (external)
- Updated navigation styling with active state highlighting

### 2. Created components.templ ✅
Reusable UI components:
- `HealthDot(status)` - Colored health indicator
- `StatCard(title, value, subtitle, trend)` - Statistics card
- `DataTable(headers, rows)` - Responsive table
- `Pagination(current, total, baseURL)` - Page navigation
- `SearchBar(placeholder, action, method)` - Search input
- `Toast(message, type)` - Notification toasts
- `Modal(id, title)` - Modal dialog
- `DropdownMenu(items)` - Dropdown menu
- `Badge(text, color)` - Colored badges

### 3. Created dashboard.templ ✅
New dashboard page at `/dashboard` (or `/`) showing:
- Quick search bar (redirects to /search)
- Health status cards (ML, Qdrant, System)
- Quick stats: Total stores, files, chunks, active connections
- Recent activity: Last 5 searches, last 5 indexed files
- System resources: Memory usage, goroutines, uptime
- Quick links panel

Data structures:
```go
type DashboardData struct {
    Layout         LayoutData
    Health         HealthSummary
    QuickStats     QuickStats
    RecentSearches []RecentSearch
    RecentFiles    []RecentFile
    System         SystemInfo
}
```

### 4. Updated Existing Templates ✅
- `search.templ` - Added LayoutData field
- `admin.templ` - Added LayoutData field
- `stats.templ` - Added LayoutData field
- `files.templ` - Added LayoutData field

## Remaining Tasks

### 1. Fix mapper_editor.templ Syntax Error ⚠️
**Error**: Line 97, col 66 - unterminated 'for' statement

**Action Required**: Edit `F:\work\rice-search\go-search\internal\web\mapper_editor.templ` - Line 97 has a syntax error with the 'for' loop. Likely needs closing brace or proper termination.

### 2. Update handlers.go 🔧

See detailed code examples in sections below.

### 3. Generate Templ Files 🔧

After fixing mapper_editor.templ:
```bash
cd F:\work\rice-search\go-search\internal\web
templ generate
```

## Implementation Guide

### A. Add Layout Data Helper

Add to `internal/web/handlers.go`:

```go
func (h *Handler) getLayoutData(ctx context.Context, currentPath string) LayoutData {
    healthStatus := "healthy"
    healthResp, err := h.grpc.Health(ctx, &pb.HealthRequest{})
    if err != nil || healthResp == nil {
        healthStatus = "unhealthy"
    } else {
        for _, comp := range healthResp.Components {
            if comp.Status == pb.HealthStatus_HEALTH_STATUS_UNHEALTHY {
                healthStatus = "unhealthy"
                break
            } else if comp.Status == pb.HealthStatus_HEALTH_STATUS_DEGRADED && healthStatus != "unhealthy" {
                healthStatus = "degraded"
            }
        }
    }
    
    version := "v1.0.0"
    verResp, err := h.grpc.Version(ctx, &pb.VersionRequest{})
    if err == nil && verResp != nil {
        version = verResp.Version
    }
    
    return LayoutData{
        Title:        "",
        CurrentPath:  currentPath,
        HealthStatus: healthStatus,
        Version:      version,
        QdrantURL:    h.qdrantURL,
    }
}
```

### B. Update Handler Struct

```go
type Handler struct {
    grpc      GRPCClient
    log       *logger.Logger
    startTime time.Time
    qdrantURL string
}

func NewHandler(grpc GRPCClient, log *logger.Logger, qdrantURL string) *Handler {
    return &Handler{
        grpc:      grpc,
        log:       log,
        startTime: time.Now(),
        qdrantURL: qdrantURL,
    }
}
```

### C. Add Dashboard Handler

```go
func (h *Handler) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    layout := h.getLayoutData(ctx, "/dashboard")
    layout.Title = "Dashboard"
    
    // Get health summary
    healthSummary := HealthSummary{Overall: layout.HealthStatus}
    healthResp, err := h.grpc.Health(ctx, &pb.HealthRequest{})
    if err == nil {
        // Parse component health...
    }
    
    // Get quick stats
    quickStats := QuickStats{}
    storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
    if err == nil {
        quickStats.TotalStores = len(storesResp.Stores)
        for _, s := range storesResp.Stores {
            if s.Stats != nil {
                quickStats.TotalFiles += s.Stats.DocumentCount
                quickStats.TotalChunks += s.Stats.ChunkCount
            }
        }
    }
    
    // System info
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    sysInfo := SystemInfo{
        MemoryUsedMB:  int64(m.Alloc / 1024 / 1024),
        MemoryTotalMB: int64(m.Sys / 1024 / 1024),
        Goroutines:    runtime.NumGoroutine(),
        Uptime:        time.Since(h.startTime).Round(time.Second).String(),
    }
    
    data := DashboardData{
        Layout:         layout,
        Health:         healthSummary,
        QuickStats:     quickStats,
        RecentSearches: []RecentSearch{},
        RecentFiles:    []RecentFile{},
        System:         sysInfo,
    }
    
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := DashboardPage(data).Render(ctx, w); err != nil {
        h.log.Error("Failed to render dashboard", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}
```

### D. Update Existing Handlers

```go
// Example for search page
func (h *Handler) handleSearchPage(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    layout := h.getLayoutData(ctx, "/search")
    layout.Title = "Search"
    
    // ... existing code ...
    
    data := SearchPageData{
        Layout: layout,
        // ... existing fields ...
    }
    
    // ... rest of code ...
}

// Similar updates for admin, stats, files pages
```

### E. Update Routes

```go
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    // Pages
    mux.HandleFunc("GET /", h.handleDashboardPage)
    mux.HandleFunc("GET /dashboard", h.handleDashboardPage)
    mux.HandleFunc("GET /search", h.handleSearchPage)
    mux.HandleFunc("GET /stores", h.handleAdminPage)
    mux.HandleFunc("GET /files", h.handleFilesPage)
    mux.HandleFunc("GET /stats", h.handleStatsPage)
    
    // Legacy routes
    mux.HandleFunc("GET /admin", h.handleAdminPage)
    
    // ... existing routes ...
}
```

### F. Update Server Initialization

```go
// In cmd/rice-search-server/main.go or server initialization
qdrantURL := cfg.Qdrant.URL
webHandler := web.NewHandler(grpcClient, logger, qdrantURL)
```

## Testing

1. **Fix mapper_editor.templ syntax error**
2. **Run templ generate**:
   ```bash
   cd internal/web && templ generate
   ```
3. **Update handlers.go** with code above
4. **Start server**:
   ```bash
   go run cmd/rice-search-server/main.go
   ```
5. **Test URLs**:
   - http://localhost:8080/ (dashboard)
   - http://localhost:8080/search
   - http://localhost:8080/stores
   - http://localhost:8080/files
   - http://localhost:8080/stats

## File Summary

### New Files:
- `internal/web/components.templ`
- `internal/web/dashboard.templ`

### Modified Files:
- `internal/web/layout.templ`
- `internal/web/search.templ`
- `internal/web/admin.templ`
- `internal/web/stats.templ`
- `internal/web/files.templ`

### Need Updates:
- `internal/web/handlers.go` (main work)
- `internal/web/mapper_editor.templ` (fix syntax)
- Server initialization (pass Qdrant URL)

## Design Notes

- **Color Scheme**: Green (healthy), Yellow (degraded), Red (unhealthy)
- **Navigation**: All pages use same header with active state
- **Responsive**: Mobile-friendly with Tailwind CSS
- **Interactive**: HTMX for dynamic updates
- **Accessibility**: Proper ARIA labels and semantic HTML

## Next Steps

1. Fix mapper_editor.templ line 97
2. Implement handler updates (copy code from sections A-F)
3. Run templ generate
4. Update server init with Qdrant URL
5. Test all pages thoroughly
