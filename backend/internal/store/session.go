package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/freewhen/backend/internal/model"
)

const (
	sessionExpiry         = 30 * 24 * time.Hour // sliding window
	sessionAbsoluteExpiry = 90 * 24 * time.Hour // hard cap from creation
)

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// CreateSession generates a crypto-random token, stores its SHA-256 hash,
// and returns the plain token for the caller to set as a cookie.
func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	token := generateToken()
	hash := hashToken(token)
	now := time.Now()
	expiresAt := now.Add(sessionExpiry)
	absoluteExpiresAt := now.Add(sessionAbsoluteExpiry)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, absolute_expires_at) VALUES ($1, $2, $3, $4)`,
		hash, userID, expiresAt, absoluteExpiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

func (s *Store) GetSession(ctx context.Context, token string) (*model.Session, error) {
	hash := hashToken(token)
	var sess model.Session
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at, absolute_expires_at, created_at FROM sessions WHERE token_hash = $1`,
		hash,
	).Scan(&sess.UserID, &sess.ExpiresAt, &sess.AbsoluteExpiresAt, &sess.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	now := time.Now()
	if now.After(sess.ExpiresAt) || now.After(sess.AbsoluteExpiresAt) {
		return nil, nil
	}
	return &sess, nil
}

// RefreshSession slides expires_at forward but clamps it to
// absolute_expires_at — the hard cap set at session creation. Once the
// sliding window would advance past the absolute cap, expires_at pins to
// the absolute and the next read after that point treats the session as
// expired, forcing re-auth.
func (s *Store) RefreshSession(ctx context.Context, token string) error {
	hash := hashToken(token)
	expiresAt := time.Now().Add(sessionExpiry)
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = LEAST($1::timestamptz, absolute_expires_at) WHERE token_hash = $2`,
		expiresAt, hash,
	)
	if err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}
	return nil
}

// PruneSessionsForUser keeps the most-recent `keep` sessions for the user
// (by created_at DESC) and deletes any beyond that. Called after
// CreateSession on login paths to bound the per-user session count.
//
// Rationale (auth audit F11): unbounded sessions let a token-theft attacker
// hold a parallel session indefinitely within the absolute-expiry window
// without colliding with the legitimate user. A cap of 10 is loose enough
// that real users on phone + laptop + work laptop + tablet won't get bumped,
// tight enough that attacker accumulation is bounded.
//
// Also closes F1's residual fixation row: a pre-set victim cookie that
// matched an existing session row gets pruned alongside legitimate old rows
// once enough fresh logins accumulate. The 90d absolute cap (F2) plus
// cookie overwrite plus this prune together bound the residual-row threat.
func (s *Store) PruneSessionsForUser(ctx context.Context, userID string, keep int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions
		 WHERE user_id = $1
		   AND token_hash NOT IN (
		       SELECT token_hash FROM sessions
		       WHERE user_id = $1
		       ORDER BY created_at DESC
		       LIMIT $2
		   )`,
		userID, keep,
	)
	if err != nil {
		return fmt.Errorf("prune sessions: %w", err)
	}
	return nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	hash := hashToken(token)
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`, hash,
	)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionsForUser deletes every session row for a user. Used by the
// "log out everywhere" flow (auth audit F12) so a user with a suspected
// stolen token can self-revoke. The caller is expected to immediately
// re-issue a fresh session for the current device after this returns.
func (s *Store) DeleteSessionsForUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = $1`, userID,
	)
	if err != nil {
		return fmt.Errorf("delete sessions for user: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < now()`,
	)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
