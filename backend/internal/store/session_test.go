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

// TestPruneSessionsForUser asserts that after creating more than `keep`
// sessions for a user, prune deletes the oldest and keeps the newest `keep`
// rows. Other users' sessions are untouched.
func TestPruneSessionsForUser(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Prune User")
	other := createTestUser(t, s, "Other User")

	// Other user's session must survive the prune of the first user.
	otherToken, err := s.CreateSession(ctx, other.ID)
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}

	// 12 sessions for u, separated by created_at so ORDER BY is stable.
	tokens := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		tok, err := s.CreateSession(ctx, u.ID)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		// Backdate older rows so created_at order matches creation order.
		if _, err := s.db.ExecContext(ctx,
			`UPDATE sessions SET created_at = $1 WHERE token_hash = $2`,
			time.Now().Add(time.Duration(-12+i)*time.Minute), hashToken(tok),
		); err != nil {
			t.Fatalf("backdate %d: %v", i, err)
		}
		tokens = append(tokens, tok)
	}

	if err := s.PruneSessionsForUser(ctx, u.ID, 10); err != nil {
		t.Fatalf("prune: %v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1`, u.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 10 {
		t.Fatalf("expected 10 sessions kept, got %d", count)
	}

	// Newest 10 (indices 2..11) should still resolve; oldest 2 (0, 1) should not.
	for i, tok := range tokens {
		sess, err := s.GetSession(ctx, tok)
		if err != nil {
			t.Fatalf("get session %d: %v", i, err)
		}
		if i < 2 && sess != nil {
			t.Fatalf("token %d (oldest) should have been pruned", i)
		}
		if i >= 2 && sess == nil {
			t.Fatalf("token %d (newest 10) should have survived", i)
		}
	}

	// Other user's session is untouched.
	if sess, err := s.GetSession(ctx, otherToken); err != nil || sess == nil {
		t.Fatalf("other user's session pruned unexpectedly: sess=%v err=%v", sess, err)
	}
}

// TestPruneSessionsForUserBelowKeep is a no-op when the user has fewer
// sessions than the cap.
func TestPruneSessionsForUserBelowKeep(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Below Keep User")

	for i := 0; i < 3; i++ {
		if _, err := s.CreateSession(ctx, u.ID); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if err := s.PruneSessionsForUser(ctx, u.ID, 10); err != nil {
		t.Fatalf("prune: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1`, u.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 sessions, got %d", count)
	}
}

// TestDeleteSessionsForUser pins the F12 contract: every session row for the
// targeted user is removed; other users' sessions are untouched.
func TestDeleteSessionsForUser(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, "Logout-All User")
	other := createTestUser(t, s, "Bystander User")

	var uTokens []string
	for i := 0; i < 4; i++ {
		tok, err := s.CreateSession(ctx, u.ID)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		uTokens = append(uTokens, tok)
	}
	otherToken, err := s.CreateSession(ctx, other.ID)
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}

	if err := s.DeleteSessionsForUser(ctx, u.ID); err != nil {
		t.Fatalf("delete sessions for user: %v", err)
	}

	for i, tok := range uTokens {
		sess, err := s.GetSession(ctx, tok)
		if err != nil {
			t.Fatalf("get session %d: %v", i, err)
		}
		if sess != nil {
			t.Fatalf("session %d should have been deleted", i)
		}
	}

	if sess, err := s.GetSession(ctx, otherToken); err != nil || sess == nil {
		t.Fatalf("other user's session deleted unexpectedly: sess=%v err=%v", sess, err)
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
