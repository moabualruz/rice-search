package ml

import (
	"testing"
)

func TestEmbeddingCache_SetGet(t *testing.T) {
	cache := NewEmbeddingCache(100)

	// Test set and get
	text := "hello world"
	embedding := []float32{0.1, 0.2, 0.3}

	cache.Set(text, embedding)

	got, ok := cache.Get(text)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if len(got) != len(embedding) {
		t.Errorf("got len %d, want %d", len(got), len(embedding))
	}

	for i := range embedding {
		if got[i] != embedding[i] {
			t.Errorf("got[%d] = %f, want %f", i, got[i], embedding[i])
		}
	}
}

func TestEmbeddingCache_Miss(t *testing.T) {
	cache := NewEmbeddingCache(100)

	_, ok := cache.Get("not in cache")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestEmbeddingCache_Eviction(t *testing.T) {
	cache := NewEmbeddingCache(3)

	// Fill cache
	cache.Set("a", []float32{1})
	cache.Set("b", []float32{2})
	cache.Set("c", []float32{3})

	// Add one more (should evict "a")
	cache.Set("d", []float32{4})

	// "a" should be evicted
	if _, ok := cache.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}

	// Others should still be present
	if _, ok := cache.Get("b"); !ok {
		t.Error("expected 'b' to be present")
	}
	if _, ok := cache.Get("c"); !ok {
		t.Error("expected 'c' to be present")
	}
	if _, ok := cache.Get("d"); !ok {
		t.Error("expected 'd' to be present")
	}
}

func TestEmbeddingCache_LRU(t *testing.T) {
	cache := NewEmbeddingCache(3)

	// Fill cache
	cache.Set("a", []float32{1})
	cache.Set("b", []float32{2})
	cache.Set("c", []float32{3})

	// Access "a" to make it recently used
	cache.Get("a")

	// Add one more (should evict "b" as LRU)
	cache.Set("d", []float32{4})

	// "a" should still be present
	if _, ok := cache.Get("a"); !ok {
		t.Error("expected 'a' to be present after LRU access")
	}

	// "b" should be evicted
	if _, ok := cache.Get("b"); ok {
		t.Error("expected 'b' to be evicted")
	}
}

func TestEmbeddingCache_Update(t *testing.T) {
	cache := NewEmbeddingCache(100)

	text := "test"
	cache.Set(text, []float32{1, 2, 3})
	cache.Set(text, []float32{4, 5, 6})

	got, ok := cache.Get(text)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if got[0] != 4 {
		t.Errorf("expected updated value, got %f", got[0])
	}

	// Size should still be 1
	if cache.Size() != 1 {
		t.Errorf("size = %d, want 1", cache.Size())
	}
}

func TestEmbeddingCache_Clear(t *testing.T) {
	cache := NewEmbeddingCache(100)

	cache.Set("a", []float32{1})
	cache.Set("b", []float32{2})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("size after clear = %d, want 0", cache.Size())
	}

	if _, ok := cache.Get("a"); ok {
		t.Error("expected cache miss after clear")
	}
}

func TestEmbeddingCache_Stats(t *testing.T) {
	cache := NewEmbeddingCache(100)

	cache.Set("a", []float32{1})
	cache.Set("b", []float32{2})

	stats := cache.Stats()

	if stats.Size != 2 {
		t.Errorf("stats.Size = %d, want 2", stats.Size)
	}

	if stats.MaxSize != 100 {
		t.Errorf("stats.MaxSize = %d, want 100", stats.MaxSize)
	}
}

func TestEmbeddingCache_ImmutableCopy(t *testing.T) {
	cache := NewEmbeddingCache(100)

	original := []float32{1, 2, 3}
	cache.Set("test", original)

	// Modify original
	original[0] = 999

	// Cache should have original value
	got, _ := cache.Get("test")
	if got[0] != 1 {
		t.Error("cache value was mutated")
	}

	// Modify returned value
	got[1] = 888

	// Cache should still have original value
	got2, _ := cache.Get("test")
	if got2[1] != 2 {
		t.Error("cache value was mutated through returned slice")
	}
}
