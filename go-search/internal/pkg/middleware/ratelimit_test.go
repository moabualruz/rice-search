package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.RequestsPerSecond != 100 {
		t.Errorf("expected RequestsPerSecond=100, got %f", cfg.RequestsPerSecond)
	}
	if cfg.Burst != 200 {
		t.Errorf("expected Burst=200, got %d", cfg.Burst)
	}
	if cfg.CleanupInterval != time.Minute {
		t.Errorf("expected CleanupInterval=1m, got %v", cfg.CleanupInterval)
	}
}

func TestNewRateLimiter(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		CleanupInterval:   10 * time.Second,
	}

	rl := NewRateLimiter(cfg)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.rate != 10 {
		t.Errorf("expected rate=10, got %f", rl.rate)
	}
	if rl.burst != 20 {
		t.Errorf("expected burst=20, got %d", rl.burst)
	}
	if len(rl.clients) != 0 {
		t.Errorf("expected empty clients map, got %d entries", len(rl.clients))
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	clientIP := "192.168.1.100"

	// First 2 requests should be allowed (burst)
	if !rl.Allow(clientIP) {
		t.Error("expected first request to be allowed")
	}
	if !rl.Allow(clientIP) {
		t.Error("expected second request to be allowed")
	}

	// Third request should be denied (burst exhausted)
	if rl.Allow(clientIP) {
		t.Error("expected third request to be denied")
	}

	// Wait for rate limit to refill
	time.Sleep(600 * time.Millisecond)

	// Should allow one more request now
	if !rl.Allow(clientIP) {
		t.Error("expected request to be allowed after waiting")
	}
}

func TestRateLimiter_MultipleClients(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             5,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	client1 := "192.168.1.100"
	client2 := "192.168.1.101"

	// Both clients should have independent limits
	for i := 0; i < 5; i++ {
		if !rl.Allow(client1) {
			t.Errorf("client1 request %d should be allowed", i)
		}
		if !rl.Allow(client2) {
			t.Errorf("client2 request %d should be allowed", i)
		}
	}

	// Both should be rate limited now
	if rl.Allow(client1) {
		t.Error("client1 should be rate limited")
	}
	if rl.Allow(client2) {
		t.Error("client2 should be rate limited")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             100,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			clientIP := "192.168.1." + string(rune('0'+clientNum))
			for j := 0; j < requestsPerGoroutine; j++ {
				rl.Allow(clientIP)
			}
		}(i)
	}

	wg.Wait()

	// Just verify no panics occurred
	t.Log("Concurrent access test passed")
}

func TestRateLimiter_Middleware(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// First 2 requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, w.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	ip := getClientIP(req)

	if ip != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", ip)
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")

	ip := getClientIP(req)

	if ip != "203.0.113.1" {
		t.Errorf("expected IP 203.0.113.1, got %s", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.50")

	ip := getClientIP(req)

	if ip != "203.0.113.50" {
		t.Errorf("expected IP 203.0.113.50, got %s", ip)
	}
}

func TestGetClientIP_HeaderPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.50")

	ip := getClientIP(req)

	// X-Forwarded-For should take precedence
	if ip != "203.0.113.1" {
		t.Errorf("expected IP 203.0.113.1 (X-Forwarded-For priority), got %s", ip)
	}
}

func TestGetClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:12345"

	ip := getClientIP(req)

	if ip != "[2001:db8::1]" {
		t.Errorf("expected IP [2001:db8::1], got %s", ip)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             100,
		CleanupInterval:   100 * time.Millisecond,
	}

	rl := NewRateLimiter(cfg)

	// Create entries for multiple clients
	for i := 0; i < 5; i++ {
		clientIP := "192.168.1." + string(rune('0'+i))
		rl.Allow(clientIP)
	}

	// Verify they exist
	rl.mu.RLock()
	initialCount := len(rl.clients)
	rl.mu.RUnlock()

	if initialCount != 5 {
		t.Errorf("expected 5 clients, got %d", initialCount)
	}

	// Wait for cleanup to run (5 minutes threshold + cleanup interval)
	// Since the threshold is 5 minutes in production, we can't easily test this
	// without mocking time. Just verify the cleanup mechanism is set up.
	time.Sleep(200 * time.Millisecond)

	// Entries should still exist (not old enough)
	rl.mu.RLock()
	afterCleanup := len(rl.clients)
	rl.mu.RUnlock()

	if afterCleanup != 5 {
		t.Errorf("expected 5 clients after cleanup (not old enough), got %d", afterCleanup)
	}
}
