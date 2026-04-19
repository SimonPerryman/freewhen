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

func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	token := generateToken()
	expiresAt := time.Now().Add(sessionExpiry)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

func (s *Store) GetSession(ctx context.Context, token string) (*model.Session, error) {
	var sess model.Session
	err := s.db.QueryRowContext(ctx,
		`SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = $1`,
		token,
	).Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt)
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
	expiresAt := time.Now().Add(sessionExpiry)
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE token = $2`,
		expiresAt, token,
	)
	if err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}
	return nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE token = $1`, token,
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
