// Package middleware provides HTTP middleware components.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	apperrors "github.com/ricesearch/rice-search/internal/pkg/errors"
)

// RateLimiter provides per-client rate limiting.
type RateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
	lastSeen map[string]time.Time
}

// RateLimiterConfig configures the rate limiter.
type RateLimiterConfig struct {
	// RequestsPerSecond is the rate limit per client.
	RequestsPerSecond float64
	// Burst is the maximum burst size.
	Burst int
	// CleanupInterval is how often to clean up stale clients.
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 100,         // 100 req/sec per client
		Burst:             200,         // Allow bursts up to 200
		CleanupInterval:   time.Minute, // Clean up every minute
	}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*rate.Limiter),
		rate:     rate.Limit(cfg.RequestsPerSecond),
		burst:    cfg.Burst,
		cleanup:  cfg.CleanupInterval,
		lastSeen: make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// getLimiter returns the rate limiter for a client, creating one if needed.
func (rl *RateLimiter) getLimiter(clientIP string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.lastSeen[clientIP] = time.Now()

	limiter, exists := rl.clients[clientIP]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.clients[clientIP] = limiter
	}

	return limiter
}

// cleanupLoop removes stale client entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		threshold := time.Now().Add(-5 * time.Minute)
		for ip, lastSeen := range rl.lastSeen {
			if lastSeen.Before(threshold) {
				delete(rl.clients, ip)
				delete(rl.lastSeen, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given IP should be allowed.
func (rl *RateLimiter) Allow(clientIP string) bool {
	return rl.getLimiter(clientIP).Allow()
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		if !rl.Allow(clientIP) {
			apperrors.WriteErrorWithStatus(w, http.StatusTooManyRequests,
				apperrors.RateLimitedError(1))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
