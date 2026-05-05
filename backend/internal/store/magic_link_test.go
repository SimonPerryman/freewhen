//go:build integration

package store

import (
	"context"
	"errors"
	"testing"
)

// TestCreateMagicLinkToken_InvalidatesPriorTokensForEmail pins the
// "only one outstanding token per email" property: issuing a new magic
// link for an email deletes any prior unused tokens for that email so
// only the most recent token is ever valid. Limits the blast radius of a
// leaked link (Slack unfurl, email forwarding) and prevents an attacker
// from accumulating many parallel valid tokens by spamming the endpoint.
func TestCreateMagicLinkToken_InvalidatesPriorTokensForEmail(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	first, err := s.CreateMagicLinkToken(ctx, "alice@example.com", "/")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	second, err := s.CreateMagicLinkToken(ctx, "alice@example.com", "/x")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct tokens")
	}

	// First token must no longer verify.
	if _, _, err := s.VerifyMagicLinkToken(ctx, first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("first token still verifies (err=%v) — invalidate-prior failed", err)
	}

	// Second still works.
	email, returnURL, err := s.VerifyMagicLinkToken(ctx, second)
	if err != nil {
		t.Fatalf("verify second: %v", err)
	}
	if email != "alice@example.com" || returnURL != "/x" {
		t.Fatalf("unexpected verify result: email=%q returnURL=%q", email, returnURL)
	}
}

// TestCreateMagicLinkToken_DifferentEmailsIndependent confirms invalidation
// is scoped by email — issuing a token for B does not affect A's token.
func TestCreateMagicLinkToken_DifferentEmailsIndependent(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	tokA, err := s.CreateMagicLinkToken(ctx, "a@example.com", "/")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := s.CreateMagicLinkToken(ctx, "b@example.com", "/"); err != nil {
		t.Fatalf("create B: %v", err)
	}

	if _, _, err := s.VerifyMagicLinkToken(ctx, tokA); err != nil {
		t.Fatalf("A's token invalidated by B's issue: %v", err)
	}
}
