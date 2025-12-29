package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Tracker tracks document hashes for deduplication and change detection.
// It maintains an in-memory map of path -> hash for each store, allowing
// quick detection of unchanged files.
type Tracker struct {
	mu     sync.RWMutex
	stores map[string]*storeTracker // store name -> tracker
}

// storeTracker tracks documents for a single store.
type storeTracker struct {
	Hashes    map[string]string    `json:"hashes"`     // path -> content hash
	IndexedAt map[string]time.Time `json:"indexed_at"` // path -> indexed time
}

// NewTracker creates a new tracker.
func NewTracker() *Tracker {
	return &Tracker{
		stores: make(map[string]*storeTracker),
	}
}

// getOrCreateStore gets or creates a store tracker.
func (t *Tracker) getOrCreateStore(store string) *storeTracker {
	t.mu.Lock()
	defer t.mu.Unlock()

	if st, ok := t.stores[store]; ok {
		return st
	}

	st := &storeTracker{
		Hashes:    make(map[string]string),
		IndexedAt: make(map[string]time.Time),
	}
	t.stores[store] = st
	return st
}

// HasHash checks if a file with the given hash is already indexed.
func (t *Tracker) HasHash(store, path, hash string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return false
	}

	existingHash, ok := st.Hashes[path]
	return ok && existingHash == hash
}

// SetHash records a file hash.
func (t *Tracker) SetHash(store, path, hash string) {
	st := t.getOrCreateStore(store)

	t.mu.Lock()
	defer t.mu.Unlock()

	st.Hashes[path] = hash
	st.IndexedAt[path] = time.Now()
}

// RemovePath removes a path from tracking.
func (t *Tracker) RemovePath(store, path string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.stores[store]
	if !ok {
		return
	}

	delete(st.Hashes, path)
	delete(st.IndexedAt, path)
}

// RemoveByPrefix removes all paths with the given prefix.
func (t *Tracker) RemoveByPrefix(store, prefix string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.stores[store]
	if !ok {
		return
	}

	for path := range st.Hashes {
		if strings.HasPrefix(path, prefix) {
			delete(st.Hashes, path)
			delete(st.IndexedAt, path)
		}
	}
}

// GetPaths returns all tracked paths for a store.
func (t *Tracker) GetPaths(store string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return nil
	}

	paths := make([]string, 0, len(st.Hashes))
	for path := range st.Hashes {
		paths = append(paths, path)
	}

	return paths
}

// GetHash returns the hash for a path.
func (t *Tracker) GetHash(store, path string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return "", false
	}

	hash, ok := st.Hashes[path]
	return hash, ok
}

// ClearStore removes all tracking data for a store.
func (t *Tracker) ClearStore(store string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.stores, store)
}

// Stats returns statistics about tracked documents.
func (t *Tracker) Stats(store string) TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return TrackerStats{Store: store}
	}

	return TrackerStats{
		Store:      store,
		TotalFiles: len(st.Hashes),
	}
}

// TrackerStats contains tracker statistics.
type TrackerStats struct {
	Store      string `json:"store"`
	TotalFiles int    `json:"total_files"`
}

// Save persists the tracker to disk.
func (t *Tracker) Save(dir string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for store, st := range t.stores {
		path := filepath.Join(dir, store+".json")
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// Load restores the tracker from disk.
func (t *Tracker) Load(dir string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil // No data to load
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		store := strings.TrimSuffix(entry.Name(), ".json")
		path := filepath.Join(dir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var st storeTracker
		if err := json.Unmarshal(data, &st); err != nil {
			return err
		}

		// Initialize maps if nil
		if st.Hashes == nil {
			st.Hashes = make(map[string]string)
		}
		if st.IndexedAt == nil {
			st.IndexedAt = make(map[string]time.Time)
		}

		t.stores[store] = &st
	}

	return nil
}

// FileInfo contains metadata about an indexed file.
type FileInfo struct {
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
	IndexedAt time.Time `json:"indexed_at"`
}

// ListFiles returns info about all files in a store.
func (t *Tracker) ListFiles(store string, page, pageSize int) ([]FileInfo, int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return nil, 0
	}

	// Convert to slice
	files := make([]FileInfo, 0, len(st.Hashes))
	for path, hash := range st.Hashes {
		indexedAt, _ := st.IndexedAt[path]
		files = append(files, FileInfo{
			Path:      path,
			Hash:      hash,
			IndexedAt: indexedAt,
		})
	}

	total := len(files)

	// Paginate
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	start := (page - 1) * pageSize
	if start >= total {
		return nil, total
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return files[start:end], total
}

// Changed returns files that have changed based on new hashes.
func (t *Tracker) Changed(store string, files map[string]string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		// No tracking data, all files are new
		result := make([]string, 0, len(files))
		for path := range files {
			result = append(result, path)
		}
		return result
	}

	var changed []string
	for path, newHash := range files {
		oldHash, exists := st.Hashes[path]
		if !exists || oldHash != newHash {
			changed = append(changed, path)
		}
	}

	return changed
}

// Removed returns files that were indexed but are no longer present.
func (t *Tracker) Removed(store string, currentPaths []string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st, ok := t.stores[store]
	if !ok {
		return nil
	}

	// Build set of current paths
	current := make(map[string]struct{}, len(currentPaths))
	for _, path := range currentPaths {
		current[path] = struct{}{}
	}

	// Find removed
	var removed []string
	for path := range st.Hashes {
		if _, exists := current[path]; !exists {
			removed = append(removed, path)
		}
	}

	return removed
}
