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

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < now()`,
	)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
