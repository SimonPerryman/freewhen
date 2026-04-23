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
