// Package ratelimit provides a token-bucket rate limiter for the control API.
// It enforces per-IP rate limits on credential generation and other endpoints
// to prevent abuse.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Limiter is a thread-safe token bucket rate limiter.
type Limiter struct {
	mu              sync.Mutex
	buckets         map[string]*bucket
	rate            float64 // tokens per second
	burst           int     // maximum bucket size
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewLimiter creates a rate limiter that allows `rate` requests per second
// with a maximum burst of `burst`.
func NewLimiter(rate float64, burst int) *Limiter {
	return &Limiter{
		buckets:         make(map[string]*bucket),
		rate:            rate,
		burst:           burst,
		cleanupInterval: 5 * time.Minute,
		lastCleanup:     time.Now(),
	}
}

// Allow checks if a request from the given key is allowed.
// Returns true if the request should proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.maybeCleanup()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{
			tokens:    float64(l.burst) - 1, // consume one immediately
			lastCheck: time.Now(),
		}
		l.buckets[key] = b
		return true
	}

	// Refill tokens.
	elapsed := time.Since(b.lastCheck).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.lastCheck = time.Now()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// Reset clears a specific key from the limiter.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Size returns the number of tracked keys.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

func (l *Limiter) maybeCleanup() {
	if time.Since(l.lastCleanup) < l.cleanupInterval {
		return
	}
	l.lastCleanup = time.Now()

	// Remove buckets that haven't been used in 2x cleanup interval.
	cutoff := time.Now().Add(-2 * l.cleanupInterval)
	for k, b := range l.buckets {
		if b.lastCheck.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}

// clientIP extracts the client IP from an HTTP request, respecting
// X-Forwarded-For when behind a proxy.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	// Strip port from RemoteAddr.
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

// Middleware returns an HTTP middleware that rate-limits requests by client IP.
func Middleware(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
