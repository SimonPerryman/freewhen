package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const magicLinkExpiry = 15 * time.Minute

// CreateMagicLinkToken generates a crypto-random token, stores its SHA-256 hash,
// and returns the plain token to embed in the magic link URL.
//
// Any prior outstanding tokens for the same email are deleted first — only
// the most recently issued link is valid at a time. This caps the concurrent
// surface (an attacker can't fan out many parallel valid links by spamming
// the endpoint) and shortens the lifetime of any link that's been
// inadvertently shared (email forwarding, Slack unfurl, AV scanner cache).
func (s *Store) CreateMagicLinkToken(ctx context.Context, email, returnURL string) (string, error) {
	token := generateToken()
	hash := hashToken(token)
	expiresAt := time.Now().Add(magicLinkExpiry)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM magic_link_tokens WHERE email = $1`, email,
	); err != nil {
		return "", fmt.Errorf("invalidate prior magic link tokens: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO magic_link_tokens (token_hash, email, return_url, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		hash, email, returnURL, expiresAt,
	); err != nil {
		return "", fmt.Errorf("insert magic link token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit magic link token: %w", err)
	}
	return token, nil
}

// VerifyMagicLinkToken looks up the token hash, checks expiry, deletes the token
// (one-time use), and returns the associated email and return URL.
func (s *Store) VerifyMagicLinkToken(ctx context.Context, plainToken string) (email, returnURL string, err error) {
	hash := hashToken(plainToken)

	var expiresAt time.Time
	err = s.db.QueryRowContext(ctx,
		`DELETE FROM magic_link_tokens WHERE token_hash = $1
		 RETURNING email, return_url, expires_at`,
		hash,
	).Scan(&email, &returnURL, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("verify magic link token: %w", err)
	}

	if time.Now().After(expiresAt) {
		return "", "", ErrNotFound
	}

	return email, returnURL, nil
}

// DeleteExpiredMagicLinkTokens removes all expired tokens.
func (s *Store) DeleteExpiredMagicLinkTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM magic_link_tokens WHERE expires_at < now()`,
	)
	if err != nil {
		return fmt.Errorf("delete expired magic link tokens: %w", err)
	}
	return nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
