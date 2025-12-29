package ml

import (
	"sync"

	"github.com/ricesearch/rice-search/internal/pkg/hash"
)

// EmbeddingCache caches embeddings by text hash.
type EmbeddingCache struct {
	mu      sync.RWMutex
	cache   map[string][]float32
	maxSize int
	order   []string // LRU order
}

// NewEmbeddingCache creates a new embedding cache.
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	if maxSize <= 0 {
		maxSize = 10000
	}

	return &EmbeddingCache{
		cache:   make(map[string][]float32),
		maxSize: maxSize,
		order:   make([]string, 0, maxSize),
	}
}

// Get retrieves an embedding from cache.
func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
	key := hash.SHA256String(text)

	c.mu.RLock()
	emb, ok := c.cache[key]
	c.mu.RUnlock()

	if ok {
		// Move to end of LRU (most recently used)
		c.mu.Lock()
		c.moveToEnd(key)
		c.mu.Unlock()

		// Return a copy to prevent external mutation
		embCopy := make([]float32, len(emb))
		copy(embCopy, emb)
		return embCopy, true
	}

	return nil, false
}

// Set stores an embedding in cache.
func (c *EmbeddingCache) Set(text string, embedding []float32) {
	key := hash.SHA256String(text)

	// Copy embedding to avoid external mutations
	embCopy := make([]float32, len(embedding))
	copy(embCopy, embedding)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if _, exists := c.cache[key]; exists {
		c.cache[key] = embCopy
		c.moveToEnd(key)
		return
	}

	// Evict if at capacity
	for len(c.cache) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.cache, oldest)
	}

	// Add new entry
	c.cache[key] = embCopy
	c.order = append(c.order, key)
}

// moveToEnd moves a key to the end of the LRU order (must hold lock).
func (c *EmbeddingCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

// Size returns the current cache size.
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Clear clears the cache.
func (c *EmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float32)
	c.order = make([]string, 0, c.maxSize)
}

// Stats returns cache statistics.
func (c *EmbeddingCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    len(c.cache),
		MaxSize: c.maxSize,
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Size    int `json:"size"`
	MaxSize int `json:"max_size"`
}
