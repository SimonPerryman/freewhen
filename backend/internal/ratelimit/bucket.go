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
	window     time.Duration // time to refill from empty to full

	mu         sync.Mutex
	tokens     map[string]float64
	updated    map[string]time.Time
	lastPruned time.Time
	now        func() time.Time
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
		window:     window,
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
	b.pruneLocked(now)
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

// WithClock replaces the bucket's time source. For tests in other packages
// that need to simulate time passing; production code never calls it.
func (b *Bucket) WithClock(now func() time.Time) *Bucket {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = now
	return b
}

// RefillInterval is how long a drained key waits for its next token. Callers
// use it to populate a Retry-After header.
func (b *Bucket) RefillInterval() time.Duration {
	return b.refillRate
}

// pruneLocked drops keys that have been idle for at least a full window. Such
// a key has refilled to capacity, which is exactly the state a missing key
// starts in, so dropping it doesn't change behaviour — it just stops a bucket
// keyed on client IPs from growing without bound. Runs at most once per
// window, so the O(keys) sweep is amortised. Caller holds b.mu.
func (b *Bucket) pruneLocked(now time.Time) {
	if b.lastPruned.IsZero() {
		b.lastPruned = now
		return
	}
	if now.Sub(b.lastPruned) < b.window {
		return
	}
	b.lastPruned = now
	for key, last := range b.updated {
		if now.Sub(last) >= b.window {
			delete(b.updated, key)
			delete(b.tokens, key)
		}
	}
}
