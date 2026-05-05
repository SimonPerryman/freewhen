//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Session User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	sess, err := s.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.UserID != u.ID {
		t.Fatalf("user id: got %q want %q", sess.UserID, u.ID)
	}
	if time.Until(sess.ExpiresAt) <= 0 {
		t.Fatalf("expected future expiry, got %v", sess.ExpiresAt)
	}
}

// TestSessionTokenStoredHashedNotPlaintext is a regression test for the
// migration that switched sessions.token (plaintext) to sessions.token_hash
// (sha256). DB row data must not contain the raw token — leaking a backup
// or row-read should not yield usable bearer credentials.
func TestSessionTokenStoredHashedNotPlaintext(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Hash User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	var stored string
	err = s.db.QueryRowContext(ctx,
		`SELECT token_hash FROM sessions WHERE user_id = $1`, u.ID,
	).Scan(&stored)
	if err != nil {
		t.Fatalf("read row: %v", err)
	}

	if stored == token {
		t.Fatal("token stored plaintext — must be hashed at rest")
	}
	if stored != hashToken(token) {
		t.Fatalf("stored value is not sha256(token): got %q want %q", stored, hashToken(token))
	}
}

func TestSessionDeleteInvalidatesGet(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Delete User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	sess, err := s.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess != nil {
		t.Fatal("expected nil session after delete")
	}
}
