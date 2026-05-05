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

// TestSessionAbsoluteExpiryCap verifies the absolute cap exists, is set
// ~90 days from creation, and is greater than the sliding expiry — the
// invariant that makes RefreshSession's clamp meaningful.
func TestSessionAbsoluteExpiryCap(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Absolute User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	sess, err := s.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session")
	}
	if !sess.AbsoluteExpiresAt.After(sess.ExpiresAt) {
		t.Fatalf("absolute (%v) must exceed sliding (%v)", sess.AbsoluteExpiresAt, sess.ExpiresAt)
	}
	expectedAbs := time.Now().Add(90 * 24 * time.Hour)
	delta := sess.AbsoluteExpiresAt.Sub(expectedAbs)
	if delta < -time.Minute || delta > time.Minute {
		t.Fatalf("absolute_expires_at = %v, expected ~%v", sess.AbsoluteExpiresAt, expectedAbs)
	}
}

// TestRefreshSessionClampsToAbsolute verifies that RefreshSession will not
// slide expires_at past absolute_expires_at. Simulate by manually setting
// absolute_expires_at to a near-future time, refreshing, and asserting
// expires_at pins to the absolute (not now+30d).
func TestRefreshSessionClampsToAbsolute(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Clamp User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	pinned := time.Now().Add(2 * time.Hour)
	if _, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET absolute_expires_at = $1 WHERE token_hash = $2`,
		pinned, hashToken(token),
	); err != nil {
		t.Fatalf("update absolute: %v", err)
	}

	if err := s.RefreshSession(ctx, token); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	sess, err := s.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session")
	}
	if !sess.ExpiresAt.Equal(sess.AbsoluteExpiresAt) {
		t.Fatalf("expires_at = %v, expected clamp to absolute %v", sess.ExpiresAt, sess.AbsoluteExpiresAt)
	}
}

// TestGetSessionRejectsPastAbsolute pins absolute_expires_at to the past
// and asserts GetSession returns nil even if expires_at is still future.
func TestGetSessionRejectsPastAbsolute(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Past Absolute User")

	token, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET absolute_expires_at = $1 WHERE token_hash = $2`,
		time.Now().Add(-time.Hour), hashToken(token),
	); err != nil {
		t.Fatalf("update absolute: %v", err)
	}

	sess, err := s.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess != nil {
		t.Fatal("expected nil session past absolute cap")
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
