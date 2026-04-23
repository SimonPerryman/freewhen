// Package ratelimit implements a small in-process token bucket keyed by an
// arbitrary string (typically user ID). Good enough for single-instance
// Cloud Run — if we ever scale horizontally this will need to move to
// Postgres or Redis.
package ratelimit

import (
	"sync"
	"time"
)

// Bucket is a per-key token bucket.
type Bucket struct {
	capacity   int
	refillRate time.Duration // time to regenerate one token

	mu      sync.Mutex
	tokens  map[string]float64
	updated map[string]time.Time
	now     func() time.Time
}

// New returns a bucket that refills to `capacity` tokens over `window`
// duration. A key that has spent all tokens will need to wait window/capacity
// before the next one is available.
func New(capacity int, window time.Duration) *Bucket {
	if capacity < 1 {
		capacity = 1
	}
	if window <= 0 {
		window = time.Hour
	}
	return &Bucket{
		capacity:   capacity,
		refillRate: window / time.Duration(capacity),
		tokens:     make(map[string]float64),
		updated:    make(map[string]time.Time),
		now:        time.Now,
	}
}

// Allow consumes one token from the key's bucket. Returns true if the request
// is permitted, false if the caller should be rejected.
func (b *Bucket) Allow(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	last, ok := b.updated[key]
	if !ok {
		b.tokens[key] = float64(b.capacity)
		last = now
	}

	elapsed := now.Sub(last)
	refill := float64(elapsed) / float64(b.refillRate)
	tokens := b.tokens[key] + refill
	if tokens > float64(b.capacity) {
		tokens = float64(b.capacity)
	}

	if tokens < 1 {
		b.tokens[key] = tokens
		b.updated[key] = now
		return false
	}

	b.tokens[key] = tokens - 1
	b.updated[key] = now
	return true
}
