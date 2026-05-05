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

const sessionExpiry = 30 * 24 * time.Hour // 30 days

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
	expiresAt := time.Now().Add(sessionExpiry)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		hash, userID, expiresAt,
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
		`SELECT user_id, expires_at, created_at FROM sessions WHERE token_hash = $1`,
		hash,
	).Scan(&sess.UserID, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, nil
	}
	return &sess, nil
}

func (s *Store) RefreshSession(ctx context.Context, token string) error {
	hash := hashToken(token)
	expiresAt := time.Now().Add(sessionExpiry)
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE token_hash = $2`,
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
