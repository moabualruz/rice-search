# Implementation Plan: CLI Watch Mode with Daemon Support

**Priority:** 🔴 P1 (Critical)  
**Effort:** Medium (2-3 days)  
**Dependencies:** None (can be done in parallel with Tree-sitter)

---

## Overview

Add file watching capability to the CLI client that monitors directories for changes and automatically syncs with the Rice Search server. Works for any file types - code, documentation, configs, logs, etc. Supports both foreground and daemon (background) modes.

## Goals

1. **Continuous sync** - Automatically index new/changed files
2. **Daemon mode** - Run in background with PID management
3. **Watcher management** - List and stop active watchers
4. **Efficient updates** - Hash-based change detection, batch uploads
5. **Gitignore support** - Respect .gitignore patterns

## CLI Interface

```bash
# Start watcher in foreground (Ctrl+C to stop)
rice-search watch ./src
rice-search watch ./src -s myproject

# Start watcher as daemon (background)
rice-search watch ./src --daemon
rice-search watch ./src -s myproject -d

# List all active watchers
rice-search watch list

# Stop specific watcher by PID
rice-search watch stop 12345

# Stop all watchers
rice-search watch stop --all

# Check watcher status
rice-search watch status 12345
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    rice-search CLI                           │
│                                                              │
│  watch ./src --daemon                                        │
│       │                                                      │
│       ▼                                                      │
│  ┌─────────────┐    fork     ┌─────────────────────────┐   │
│  │ CLI Process │───────────▶│ Daemon Process (detached)│   │
│  └─────────────┘             │                          │   │
│       │                      │  ┌────────────────────┐ │   │
│       │ write PID            │  │   fsnotify watcher │ │   │
│       ▼                      │  └─────────┬──────────┘ │   │
│  ~/.local/state/             │            │            │   │
│    rice-search/              │            ▼            │   │
│      watchers/               │  ┌────────────────────┐ │   │
│        12345.json            │  │   Batch Processor  │ │   │
│                              │  └─────────┬──────────┘ │   │
│                              │            │            │   │
│                              │            ▼            │   │
│                              │  ┌────────────────────┐ │   │
│                              │  │   HTTP Client      │ │   │
│                              │  │   → API Server     │ │   │
│                              │  └────────────────────┘ │   │
│                              └─────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Package Structure

```
cmd/rice-search/
├── main.go
├── watch.go           # watch command group
├── watch_start.go     # watch (start) command
├── watch_list.go      # watch list command
├── watch_stop.go      # watch stop command
├── watch_status.go    # watch status command

internal/
├── watch/
│   ├── watcher.go     # File system watcher
│   ├── daemon.go      # Daemon process management
│   ├── pidfile.go     # PID file operations
│   ├── state.go       # Watcher state persistence
│   ├── ignore.go      # Gitignore parsing
│   ├── hasher.go      # File hashing (xxhash)
│   └── batch.go       # Batch upload logic
├── client/
│   └── client.go      # (existing) HTTP client
```

## Implementation Steps

### Step 1: Define Watch State

**File:** `internal/watch/state.go`
```go
package watch

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

// WatcherState represents a running watcher
type WatcherState struct {
    PID          int       `json:"pid"`
    Store        string    `json:"store"`
    Path         string    `json:"path"`
    ConnectionID string    `json:"connection_id"`
    StartedAt    time.Time `json:"started_at"`
    FileCount    int       `json:"file_count"`
    LastSync     time.Time `json:"last_sync"`
}

// StateDir returns the directory for watcher state files
func StateDir() string {
    // XDG_STATE_HOME or ~/.local/state
    if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
        return filepath.Join(dir, "rice-search", "watchers")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".local", "state", "rice-search", "watchers")
}

// StatePath returns the path for a specific watcher's state file
func StatePath(pid int) string {
    return filepath.Join(StateDir(), fmt.Sprintf("%d.json", pid))
}

// SaveState saves watcher state to disk
func SaveState(state *WatcherState) error {
    dir := StateDir()
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(StatePath(state.PID), data, 0644)
}

// LoadState loads watcher state from disk
func LoadState(pid int) (*WatcherState, error) {
    data, err := os.ReadFile(StatePath(pid))
    if err != nil {
        return nil, err
    }
    
    var state WatcherState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }
    
    return &state, nil
}

// ListStates returns all watcher states
func ListStates() ([]*WatcherState, error) {
    dir := StateDir()
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    
    var states []*WatcherState
    for _, entry := range entries {
        if filepath.Ext(entry.Name()) != ".json" {
            continue
        }
        
        data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
        if err != nil {
            continue
        }
        
        var state WatcherState
        if err := json.Unmarshal(data, &state); err != nil {
            continue
        }
        
        // Check if process is still running
        if !isProcessRunning(state.PID) {
            // Clean up stale state file
            os.Remove(filepath.Join(dir, entry.Name()))
            continue
        }
        
        states = append(states, &state)
    }
    
    return states, nil
}

// RemoveState removes a watcher's state file
func RemoveState(pid int) error {
    return os.Remove(StatePath(pid))
}
```

### Step 2: Implement File Watcher

**File:** `internal/watch/watcher.go`
```go
package watch

import (
    "context"
    "log/slog"
    "os"
    "path/filepath"
    "sync"
    "time"
    
    "github.com/fsnotify/fsnotify"
    "github.com/ricesearch/go-search/internal/client"
)

type Watcher struct {
    path         string
    store        string
    client       *client.Client
    ignore       *IgnoreFilter
    
    // Batch processing
    pendingMu    sync.Mutex
    pendingFiles map[string]struct{}
    batchTimer   *time.Timer
    batchDelay   time.Duration
    
    // Stats
    fileCount    int
    lastSync     time.Time
    
    // Lifecycle
    done         chan struct{}
    log          *slog.Logger
}

type WatcherConfig struct {
    Path         string
    Store        string
    Client       *client.Client
    BatchDelay   time.Duration  // Default: 500ms
    MaxBatchSize int            // Default: 50
}

func NewWatcher(cfg WatcherConfig) (*Watcher, error) {
    if cfg.BatchDelay == 0 {
        cfg.BatchDelay = 500 * time.Millisecond
    }
    
    ignore, err := NewIgnoreFilter(cfg.Path)
    if err != nil {
        return nil, err
    }
    
    return &Watcher{
        path:         cfg.Path,
        store:        cfg.Store,
        client:       cfg.Client,
        ignore:       ignore,
        pendingFiles: make(map[string]struct{}),
        batchDelay:   cfg.BatchDelay,
        done:         make(chan struct{}),
        log:          slog.Default().With("component", "watcher"),
    }, nil
}

func (w *Watcher) Start(ctx context.Context) error {
    // Initial sync
    if err := w.initialSync(ctx); err != nil {
        return fmt.Errorf("initial sync failed: %w", err)
    }
    
    // Create fsnotify watcher
    fsWatcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer fsWatcher.Close()
    
    // Add directories recursively
    err = filepath.WalkDir(w.path, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            if w.ignore.ShouldIgnore(path) {
                return filepath.SkipDir
            }
            return fsWatcher.Add(path)
        }
        return nil
    })
    if err != nil {
        return err
    }
    
    w.log.Info("Watching for changes", "path", w.path)
    
    // Event loop
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-w.done:
            return nil
        case event, ok := <-fsWatcher.Events:
            if !ok {
                return nil
            }
            w.handleEvent(event)
        case err, ok := <-fsWatcher.Errors:
            if !ok {
                return nil
            }
            w.log.Error("Watcher error", "error", err)
        }
    }
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
    path := event.Name
    
    // Ignore if matches gitignore
    if w.ignore.ShouldIgnore(path) {
        return
    }
    
    // Add to pending batch
    w.pendingMu.Lock()
    defer w.pendingMu.Unlock()
    
    switch {
    case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
        w.pendingFiles[path] = struct{}{}
    case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
        // Mark for deletion
        w.pendingFiles[path] = struct{}{}
    }
    
    // Reset batch timer
    if w.batchTimer != nil {
        w.batchTimer.Stop()
    }
    w.batchTimer = time.AfterFunc(w.batchDelay, w.processBatch)
}

func (w *Watcher) processBatch() {
    w.pendingMu.Lock()
    files := make([]string, 0, len(w.pendingFiles))
    for path := range w.pendingFiles {
        files = append(files, path)
    }
    w.pendingFiles = make(map[string]struct{})
    w.pendingMu.Unlock()
    
    if len(files) == 0 {
        return
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Separate creates/updates from deletes
    var toIndex, toDelete []string
    for _, path := range files {
        if _, err := os.Stat(path); os.IsNotExist(err) {
            toDelete = append(toDelete, path)
        } else {
            toIndex = append(toIndex, path)
        }
    }
    
    // Index new/changed files
    if len(toIndex) > 0 {
        if err := w.indexFiles(ctx, toIndex); err != nil {
            w.log.Error("Failed to index files", "count", len(toIndex), "error", err)
        } else {
            w.log.Info("Indexed files", "count", len(toIndex))
        }
    }
    
    // Delete removed files
    if len(toDelete) > 0 {
        if err := w.deleteFiles(ctx, toDelete); err != nil {
            w.log.Error("Failed to delete files", "count", len(toDelete), "error", err)
        } else {
            w.log.Info("Deleted files", "count", len(toDelete))
        }
    }
    
    w.lastSync = time.Now()
}

func (w *Watcher) Stop() {
    close(w.done)
}

func (w *Watcher) Stats() (fileCount int, lastSync time.Time) {
    return w.fileCount, w.lastSync
}
```

### Step 3: Implement Daemon Mode

**File:** `internal/watch/daemon.go`
```go
package watch

import (
    "fmt"
    "os"
    "os/exec"
    "syscall"
)

// StartDaemon starts the watcher as a background daemon
func StartDaemon(path, store string) (int, error) {
    // Get the current executable
    exe, err := os.Executable()
    if err != nil {
        return 0, err
    }
    
    // Build args for daemon process
    args := []string{"watch", path, "-s", store, "--foreground"}
    
    // Create command
    cmd := exec.Command(exe, args...)
    
    // Detach from parent
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,  // Create new process group
        Pgid:    0,
    }
    
    // Redirect output to log file
    logDir := StateDir()
    os.MkdirAll(logDir, 0755)
    logFile, err := os.OpenFile(
        filepath.Join(logDir, fmt.Sprintf("%d.log", os.Getpid())),
        os.O_CREATE|os.O_WRONLY|os.O_APPEND,
        0644,
    )
    if err != nil {
        return 0, err
    }
    cmd.Stdout = logFile
    cmd.Stderr = logFile
    
    // Start the daemon
    if err := cmd.Start(); err != nil {
        return 0, err
    }
    
    // Don't wait for the process
    go cmd.Wait()
    
    return cmd.Process.Pid, nil
}

// StopDaemon stops a watcher daemon by PID
func StopDaemon(pid int) error {
    process, err := os.FindProcess(pid)
    if err != nil {
        return err
    }
    
    // Send SIGTERM for graceful shutdown
    if err := process.Signal(syscall.SIGTERM); err != nil {
        // Try SIGKILL if SIGTERM fails
        return process.Kill()
    }
    
    // Remove state file
    RemoveState(pid)
    
    return nil
}

// StopAllDaemons stops all running watcher daemons
func StopAllDaemons() (int, error) {
    states, err := ListStates()
    if err != nil {
        return 0, err
    }
    
    stopped := 0
    for _, state := range states {
        if err := StopDaemon(state.PID); err == nil {
            stopped++
        }
    }
    
    return stopped, nil
}

// isProcessRunning checks if a process is still running
func isProcessRunning(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    
    // On Unix, FindProcess always succeeds, so we need to send signal 0
    err = process.Signal(syscall.Signal(0))
    return err == nil
}
```

### Step 4: Implement CLI Commands

**File:** `cmd/rice-search/watch.go`
```go
package main

import (
    "fmt"
    "os"
    "text/tabwriter"
    "time"
    
    "github.com/spf13/cobra"
    "github.com/ricesearch/go-search/internal/watch"
    "github.com/ricesearch/go-search/internal/client"
)

var watchCmd = &cobra.Command{
    Use:   "watch <path>",
    Short: "Watch directory for changes and sync with server",
    Long: `Watch a directory for file changes and automatically sync with the Rice Search server.
    
Examples:
  rice-search watch ./src                    # Watch in foreground
  rice-search watch ./src --daemon           # Watch in background
  rice-search watch ./src -s myproject -d    # Background with custom store`,
    Args: cobra.ExactArgs(1),
    RunE: runWatch,
}

var (
    watchStore      string
    watchDaemon     bool
    watchForeground bool  // Internal flag for daemon child process
)

func init() {
    watchCmd.Flags().StringVarP(&watchStore, "store", "s", "default", "Store name")
    watchCmd.Flags().BoolVarP(&watchDaemon, "daemon", "d", false, "Run as background daemon")
    watchCmd.Flags().BoolVar(&watchForeground, "foreground", false, "Internal: run in foreground (used by daemon)")
    watchCmd.Flags().MarkHidden("foreground")
    
    // Subcommands
    watchCmd.AddCommand(watchListCmd)
    watchCmd.AddCommand(watchStopCmd)
    watchCmd.AddCommand(watchStatusCmd)
    
    rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
    path := args[0]
    
    // Validate path exists
    if _, err := os.Stat(path); err != nil {
        return fmt.Errorf("path does not exist: %s", path)
    }
    
    // Daemon mode: fork and exit
    if watchDaemon && !watchForeground {
        pid, err := watch.StartDaemon(path, watchStore)
        if err != nil {
            return fmt.Errorf("failed to start daemon: %w", err)
        }
        fmt.Printf("Watcher started with PID %d\n", pid)
        fmt.Printf("Stop with: rice-search watch stop %d\n", pid)
        return nil
    }
    
    // Foreground mode: run watcher directly
    c := client.New(client.Config{
        BaseURL: serverURL,
        Store:   watchStore,
    })
    
    w, err := watch.NewWatcher(watch.WatcherConfig{
        Path:   path,
        Store:  watchStore,
        Client: c,
    })
    if err != nil {
        return err
    }
    
    // Save state for daemon mode
    if watchForeground {
        state := &watch.WatcherState{
            PID:       os.Getpid(),
            Store:     watchStore,
            Path:      path,
            StartedAt: time.Now(),
        }
        watch.SaveState(state)
        defer watch.RemoveState(state.PID)
    }
    
    // Handle signals
    ctx, cancel := signal.NotifyContext(context.Background(), 
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    
    return w.Start(ctx)
}

// watch list command
var watchListCmd = &cobra.Command{
    Use:   "list",
    Short: "List active watchers",
    RunE: func(cmd *cobra.Command, args []string) error {
        states, err := watch.ListStates()
        if err != nil {
            return err
        }
        
        if len(states) == 0 {
            fmt.Println("No active watchers")
            return nil
        }
        
        w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "PID\tSTORE\tPATH\tSTARTED\tFILES")
        for _, s := range states {
            fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n",
                s.PID,
                s.Store,
                s.Path,
                s.StartedAt.Format("2006-01-02 15:04"),
                s.FileCount,
            )
        }
        return w.Flush()
    },
}

// watch stop command
var watchStopCmd = &cobra.Command{
    Use:   "stop [pid]",
    Short: "Stop a watcher",
    RunE: func(cmd *cobra.Command, args []string) error {
        all, _ := cmd.Flags().GetBool("all")
        
        if all {
            count, err := watch.StopAllDaemons()
            if err != nil {
                return err
            }
            fmt.Printf("Stopped %d watcher(s)\n", count)
            return nil
        }
        
        if len(args) == 0 {
            return fmt.Errorf("specify PID or use --all")
        }
        
        pid, err := strconv.Atoi(args[0])
        if err != nil {
            return fmt.Errorf("invalid PID: %s", args[0])
        }
        
        if err := watch.StopDaemon(pid); err != nil {
            return err
        }
        
        fmt.Printf("Stopped watcher %d\n", pid)
        return nil
    },
}

func init() {
    watchStopCmd.Flags().Bool("all", false, "Stop all watchers")
}
```

### Step 5: Implement Gitignore Support

**File:** `internal/watch/ignore.go`
```go
package watch

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"
    
    "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type IgnoreFilter struct {
    root     string
    patterns []gitignore.Pattern
}

func NewIgnoreFilter(root string) (*IgnoreFilter, error) {
    f := &IgnoreFilter{root: root}
    
    // Add default patterns
    defaultPatterns := []string{
        ".git",
        "node_modules",
        "__pycache__",
        "*.pyc",
        ".DS_Store",
        "*.lock",
        "*.log",
        "vendor",
        "dist",
        "build",
        ".idea",
        ".vscode",
    }
    
    for _, p := range defaultPatterns {
        pattern := gitignore.ParsePattern(p, nil)
        f.patterns = append(f.patterns, pattern)
    }
    
    // Load .gitignore if exists
    gitignorePath := filepath.Join(root, ".gitignore")
    if file, err := os.Open(gitignorePath); err == nil {
        defer file.Close()
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            line := strings.TrimSpace(scanner.Text())
            if line == "" || strings.HasPrefix(line, "#") {
                continue
            }
            pattern := gitignore.ParsePattern(line, nil)
            f.patterns = append(f.patterns, pattern)
        }
    }
    
    // Load .riceignore if exists
    riceignorePath := filepath.Join(root, ".riceignore")
    if file, err := os.Open(riceignorePath); err == nil {
        defer file.Close()
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            line := strings.TrimSpace(scanner.Text())
            if line == "" || strings.HasPrefix(line, "#") {
                continue
            }
            pattern := gitignore.ParsePattern(line, nil)
            f.patterns = append(f.patterns, pattern)
        }
    }
    
    return f, nil
}

func (f *IgnoreFilter) ShouldIgnore(path string) bool {
    relPath, err := filepath.Rel(f.root, path)
    if err != nil {
        return false
    }
    
    // Check each pattern
    pathParts := strings.Split(relPath, string(filepath.Separator))
    for _, pattern := range f.patterns {
        if pattern.Match(pathParts, false) == gitignore.Exclude {
            return true
        }
    }
    
    return false
}
```

## Testing Strategy

```go
func TestWatcher_FileChanges(t *testing.T) {
    // Create temp directory
    dir := t.TempDir()
    
    // Start watcher
    w, err := watch.NewWatcher(watch.WatcherConfig{
        Path:  dir,
        Store: "test",
        // Mock client
    })
    require.NoError(t, err)
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go w.Start(ctx)
    
    // Create a file
    os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0644)
    
    // Wait for batch processing
    time.Sleep(time.Second)
    
    // Verify file was indexed
    // ...
}

func TestDaemon_StartStop(t *testing.T) {
    // Start daemon
    pid, err := watch.StartDaemon("/tmp/test", "default")
    require.NoError(t, err)
    require.Greater(t, pid, 0)
    
    // Verify it's running
    assert.True(t, watch.isProcessRunning(pid))
    
    // Stop it
    err = watch.StopDaemon(pid)
    require.NoError(t, err)
    
    // Verify it stopped
    time.Sleep(100 * time.Millisecond)
    assert.False(t, watch.isProcessRunning(pid))
}
```

## Success Metrics

- [ ] `watch` command starts file watcher
- [ ] `watch --daemon` runs in background
- [ ] `watch list` shows active watchers
- [ ] `watch stop <pid>` stops specific watcher
- [ ] `watch stop --all` stops all watchers
- [ ] Respects .gitignore patterns
- [ ] Batch uploads (not per-file)
- [ ] Graceful shutdown on SIGTERM
- [ ] State persists across restarts
- [ ] Works on Linux, macOS, Windows

## References

- Old implementation: `ricegrep/src/commands/watch.ts`
- fsnotify: https://github.com/fsnotify/fsnotify
- go-git gitignore: https://github.com/go-git/go-git
