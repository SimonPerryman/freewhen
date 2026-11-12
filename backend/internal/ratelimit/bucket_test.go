package ratelimit

import (
	"testing"
	"time"
)

func TestBucketAllowsBurstUpToCapacity(t *testing.T) {
	b := New(3, time.Hour)
	for i := 0; i < 3; i++ {
		if !b.Allow("alice") {
			t.Fatalf("call %d should be allowed", i)
		}
	}
	if b.Allow("alice") {
		t.Fatalf("4th call should be rejected")
	}
}

func TestBucketRefillsOverTime(t *testing.T) {
	b := New(2, 2*time.Second)
	now := time.Unix(0, 0)
	b.now = func() time.Time { return now }

	if !b.Allow("bob") {
		t.Fatalf("first call should allow")
	}
	if !b.Allow("bob") {
		t.Fatalf("second call should allow")
	}
	if b.Allow("bob") {
		t.Fatalf("third should reject")
	}
	now = now.Add(1 * time.Second) // one token worth
	if !b.Allow("bob") {
		t.Fatalf("after refill, should allow")
	}
}

func TestBucketIsolatesKeys(t *testing.T) {
	b := New(1, time.Hour)
	if !b.Allow("a") {
		t.Fatalf("a's first call should allow")
	}
	if !b.Allow("b") {
		t.Fatalf("b's first call should allow")
	}
	if b.Allow("a") {
		t.Fatalf("a's second call should reject")
	}
}

func TestBucketPrunesIdleKeys(t *testing.T) {
	b := New(2, time.Minute)
	now := time.Unix(0, 0)
	b.now = func() time.Time { return now }

	b.Allow("idle")
	b.Allow("idle")
	if b.Allow("idle") {
		t.Fatal("idle key should be drained")
	}
	now = now.Add(30 * time.Second)
	b.Allow("active")

	// A full window after "idle" was last touched it has refilled, so the
	// next call's sweep drops it; "active" was touched more recently and stays.
	now = now.Add(31 * time.Second)
	b.Allow("trigger")
	if _, ok := b.updated["idle"]; ok {
		t.Fatal("expected idle key to be pruned")
	}
	if _, ok := b.updated["active"]; !ok {
		t.Fatal("expected recently used key to be kept")
	}
	// A pruned key behaves exactly like a full bucket.
	for i := 0; i < 2; i++ {
		if !b.Allow("idle") {
			t.Fatalf("pruned key should start with a full bucket (call %d)", i+1)
		}
	}
	if b.Allow("idle") {
		t.Fatal("pruned key should still be capped at capacity")
	}
}

func TestBucketRefillInterval(t *testing.T) {
	if got := New(120, time.Minute).RefillInterval(); got != 500*time.Millisecond {
		t.Fatalf("expected 500ms, got %v", got)
	}
}
