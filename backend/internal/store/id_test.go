package store

import (
	"regexp"
	"strings"
	"testing"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+$`)

func TestGenerateID(t *testing.T) {
	for _, n := range []int{1, 8, 10, CalendarIDLength, 64} {
		id := generateID(n)
		if len(id) != n {
			t.Fatalf("generateID(%d): expected length %d, got %d", n, n, len(id))
		}
		if !idPattern.MatchString(id) {
			t.Fatalf("generateID(%d): expected lowercase alphanumeric, got %q", n, id)
		}
	}
}

func TestCalendarIDLength(t *testing.T) {
	if CalendarIDLength < 12 {
		t.Fatalf("calendar IDs must be at least 12 chars (~62 bits), got %d", CalendarIDLength)
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

func TestMaxUnbiasedByte(t *testing.T) {
	if maxUnbiasedByte%len(alphabet) != 0 || maxUnbiasedByte > 256 || 256-maxUnbiasedByte >= len(alphabet) {
		t.Fatalf("maxUnbiasedByte %d is not the largest multiple of %d <= 256", maxUnbiasedByte, len(alphabet))
	}
}

// TestGenerateIDDistribution is a chi-squared sanity check that every
// character of the alphabet is drawn uniformly. With the old `b % 36`
// mapping, the first 4 characters were ~14% over-represented; this test would
// catch that with very high probability.
func TestGenerateIDDistribution(t *testing.T) {
	const samples = 360_000
	counts := make(map[rune]int, len(alphabet))
	var sb strings.Builder
	for sb.Len() < samples {
		sb.WriteString(generateID(CalendarIDLength))
	}
	s := sb.String()[:samples]
	for _, r := range s {
		counts[r]++
	}
	if len(counts) != len(alphabet) {
		t.Fatalf("expected all %d characters to appear, got %d", len(alphabet), len(counts))
	}
	expected := float64(samples) / float64(len(alphabet))
	var chi2 float64
	for _, r := range alphabet {
		d := float64(counts[r]) - expected
		chi2 += d * d / expected
	}
	// 35 degrees of freedom: p=0.0001 critical value is ~74.7. A biased
	// generator (old modulo mapping) scores in the hundreds at this sample
	// size.
	if chi2 > 80 {
		t.Fatalf("character distribution looks non-uniform: chi2=%.1f counts=%v", chi2, counts)
	}
}
