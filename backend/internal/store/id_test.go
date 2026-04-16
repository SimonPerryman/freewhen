package store

import (
	"regexp"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id := generateID(8)
	if len(id) != 8 {
		t.Fatalf("expected length 8, got %d", len(id))
	}
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(id) {
		t.Fatalf("expected alphanumeric lowercase, got %q", id)
	}
}

func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		ids[generateID(8)] = struct{}{}
	}
	if len(ids) != 1000 {
		t.Fatalf("expected 1000 unique IDs, got %d", len(ids))
	}
}
