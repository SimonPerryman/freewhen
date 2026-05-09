package notifications

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-padding-padding-padding-padding"

func TestTokenRoundTrip(t *testing.T) {
	s, err := NewTokenSigner(testSecret, time.Hour)
	if err != nil {
		t.Fatalf("NewTokenSigner: %v", err)
	}
	tok := s.Sign("p_abc123")
	id, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id != "p_abc123" {
		t.Fatalf("got %q, want p_abc123", id)
	}
}

func TestTamperedSignatureRejected(t *testing.T) {
	s, _ := NewTokenSigner(testSecret, time.Hour)
	tok := s.Sign("p_abc")
	dot := strings.LastIndex(tok, ".")
	// Replace the entire mac with a syntactically valid but bogus value.
	tampered := tok[:dot+1] + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := s.Verify(tampered); err == nil {
		t.Fatalf("expected verification failure for tampered signature")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	s, _ := NewTokenSigner(testSecret, -time.Second)
	tok := s.Sign("p_abc")
	if _, err := s.Verify(tok); err == nil {
		t.Fatalf("expected verification failure for expired token")
	}
}

func TestMalformedTokenRejected(t *testing.T) {
	s, _ := NewTokenSigner(testSecret, time.Hour)
	for _, tok := range []string{"", "no-dot", "a.b.c", strings.Repeat("x", 64)} {
		if _, err := s.Verify(tok); err == nil {
			t.Fatalf("expected error for token %q", tok)
		}
	}
}

func TestSecretTooShortRejected(t *testing.T) {
	if _, err := NewTokenSigner("short", time.Hour); err == nil {
		t.Fatalf("expected error for short secret")
	}
}

