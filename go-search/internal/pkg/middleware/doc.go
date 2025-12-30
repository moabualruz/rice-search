// Package middleware provides HTTP middleware components for the Rice Search server.
//
// Available middleware:
//   - RateLimiter: Per-client rate limiting using token bucket algorithm
//
// Usage:
//
//	rl := middleware.NewRateLimiter(middleware.DefaultRateLimiterConfig())
//	handler = rl.Middleware(handler)
package middleware
