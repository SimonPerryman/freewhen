package passphrase

import (
	"strings"
	"testing"
)

func TestGenerate_ShapeAndWords(t *testing.T) {
	p := Generate()
	parts := strings.Split(p, "-")
	if len(parts) != WordCount {
		t.Fatalf("generated passphrase %q has %d parts, want %d", p, len(parts), WordCount)
	}
	allowed := map[string]bool{}
	for _, w := range words {
		allowed[w] = true
	}
	for _, part := range parts {
		if !allowed[part] {
			t.Fatalf("generated passphrase contains unknown word %q", part)
		}
	}
}

func TestGenerate_NoCollisionsSmokeTest(t *testing.T) {
	// Cheap entropy smoke test: 1000 generations should not produce a
	// collision with a wordlist of ~1000 words^4 possibilities.
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		p := Generate()
		if seen[p] {
			t.Fatalf("collision after %d generations: %q", i, p)
		}
		seen[p] = true
	}
}
