package watch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ricesearch/rice-search/internal/grpcclient"
)

type Watcher struct {
	path   string
	store  string
	client *grpcclient.Client
	ignore *IgnoreFilter

	// Batch processing
	pendingMu    sync.Mutex
	pendingFiles map[string]struct{}
	batchTimer   *time.Timer
	batchDelay   time.Duration

	// Stats
	fileCount int
	lastSync  time.Time

	// Lifecycle
	done chan struct{}
	log  *slog.Logger
}

type WatcherConfig struct {
	Path       string
	Store      string
	Client     *grpcclient.Client
	BatchDelay time.Duration // Default: 500ms
}

func NewWatcher(cfg WatcherConfig) (*Watcher, error) {
	if cfg.BatchDelay == 0 {
		cfg.BatchDelay = 500 * time.Millisecond
	}

	ignore, err := NewIgnoreFilter(cfg.Path)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, err
	}

	return &Watcher{
		path:         absPath,
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
	w.log.Info("Starting watcher", "path", w.path, "store", w.store)

	// Initial sync
	if err := w.initialSync(ctx); err != nil {
		w.log.Error("Initial sync failed", "error", err)
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
			w.log.Warn("Error walking path", "path", path, "error", err)
			return filepath.SkipDir
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
			w.handleEvent(event, fsWatcher) // Pass fsWatcher to handle new dirs
		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.log.Error("Watcher error", "error", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event, fsWatcher *fsnotify.Watcher) {
	path := event.Name

	// Ignore if matches gitignore
	if w.ignore.ShouldIgnore(path) {
		return
	}

	// If it's a new directory, add it to watcher
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			fsWatcher.Add(path)
		}
	}

	// Add to pending batch
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	switch {
	case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
		w.pendingFiles[path] = struct{}{}
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		// Mark for deletion/re-indexing
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

	w.log.Info("Processing batch", "count", len(files))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Separate creates/updates from deletes
	var toIndex []grpcclient.IndexDocument
	// var toDelete []string

	for _, path := range files {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			// Deleted file
		} else if !info.IsDir() {
			// Update/Create
			content, err := os.ReadFile(path)
			if err != nil {
				w.log.Warn("Failed to read file", "path", path, "error", err)
				continue
			}

			// Ideally relative path for ID?
			relPath, _ := filepath.Rel(w.path, path)
			if relPath == "" {
				relPath = path
			}

			// Normalize separators for consistent IDs
			relPath = filepath.ToSlash(relPath)

			toIndex = append(toIndex, grpcclient.IndexDocument{
				Path:    relPath,
				Content: string(content),
				// Language field optional (autodetected by server)
			})
		}
	}

	if len(toIndex) > 0 {
		result, err := w.client.Index(ctx, w.store, toIndex, false)
		if err != nil {
			w.log.Error("Failed to index batch", "error", err)
		} else {
			w.log.Info("Batch sync complete", "indexed", result.Indexed, "failed", result.Failed)
			w.fileCount += result.Indexed
			w.lastSync = time.Now()
		}
	}
}

func (w *Watcher) initialSync(ctx context.Context) error {
	w.log.Info("Performing initial sync...")

	var batch []grpcclient.IndexDocument
	batchSize := 50 // sensible default

	err := filepath.WalkDir(w.path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // finish walk even if some errors
		}
		if d.IsDir() {
			if w.ignore.ShouldIgnore(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if w.ignore.ShouldIgnore(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(w.path, path)
		relPath = filepath.ToSlash(relPath)

		batch = append(batch, grpcclient.IndexDocument{
			Path:    relPath,
			Content: string(content),
		})

		if len(batch) >= batchSize {
			if err := w.sendBatch(ctx, batch); err != nil {
				w.log.Error("Failed to send initial batch", "error", err)
			}
			batch = nil // clear
		}

		return nil
	})

	if len(batch) > 0 {
		if err := w.sendBatch(ctx, batch); err != nil {
			w.log.Error("Failed to send final initial batch", "error", err)
		}
	}

	w.log.Info("Initial sync finished")
	return err
}

func (w *Watcher) sendBatch(ctx context.Context, files []grpcclient.IndexDocument) error {
	res, err := w.client.Index(ctx, w.store, files, false)
	if err != nil {
		return err
	}
	w.fileCount += res.Indexed
	w.lastSync = time.Now()
	return nil
}

func (w *Watcher) Stop() {
	close(w.done)
}

func (w *Watcher) Stats() (int, time.Time) {
	return w.fileCount, w.lastSync
}
