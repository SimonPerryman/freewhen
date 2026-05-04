//go:build integration

package store

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := testdb.Start(t)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := Migrate(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Clean tables before each test (order matters for FK constraints)
	for _, table := range []string{"availability", "calendar_invites", "participants", "sessions", "magic_link_tokens", "calendars", "users"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}

	return New(db)
}

// createTestUser creates a user for tests that need calendar ownership.
func createTestUser(t *testing.T, s *Store, name string) model.User {
	t.Helper()
	ctx := context.Background()

	id := generateID(8)
	email := id + "@test.example.com"
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (id, email, name, provider, provider_id)
		 VALUES ($1, $2, $3, 'test', '')
		 RETURNING id, email, name`,
		id, email, name,
	).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return u
}
