package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/freewhen/backend/internal/model"
)

func (s *Store) CreateUser(ctx context.Context, email, name, provider, providerID string) (model.User, error) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		id := generateID(8)
		var u model.User
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO users (id, email, name, provider, provider_id)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, email, name, is_admin, provider, provider_id, created_at`,
			id, email, name, provider, providerID,
		).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Provider, &u.ProviderID, &u.CreatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return model.User{}, fmt.Errorf("insert user: %w", err)
		}
		return u, nil
	}
	return model.User{}, fmt.Errorf("failed to generate unique user ID after %d attempts", maxRetries)
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, is_admin, provider, provider_id, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Provider, &u.ProviderID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, is_admin, provider, provider_id, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Provider, &u.ProviderID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *Store) FindUserByProviderID(ctx context.Context, provider, providerID string) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, is_admin, provider, provider_id, created_at FROM users WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Provider, &u.ProviderID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("find user by provider ID: %w", err)
	}
	return u, nil
}
