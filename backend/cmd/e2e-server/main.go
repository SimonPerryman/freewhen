// e2e-server boots the regular HTTP server against an embedded Postgres
// instance, with E2E_TEST_MODE enabled. It exists so Playwright's webServer
// block can launch a real backend without local Postgres or Docker.
//
// Defaults are baked in here rather than read from env so the launcher works
// from a clean shell. Override PORT and E2E_BACKEND_DATA_DIR if needed.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/embeddedpg"
	"github.com/freewhen/backend/internal/server"
	"github.com/freewhen/backend/internal/store"
)

const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("e2e-server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Stop() stops Postgres and deletes its temp dir on a normal or
	// SIGINT/SIGTERM exit. If the process is killed outright (taskkill /F,
	// Playwright tearing down `go run`), the Windows job object takes the
	// postmaster down with us and the next start reaps the leftover dir.
	pg, err := embeddedpg.Start(embeddedpg.PrefixE2E, "freewhen_e2e")
	if err != nil {
		return err
	}
	defer func() {
		if err := pg.Stop(); err != nil {
			slog.Error("stop embedded postgres", "error", err)
		}
	}()
	dsn, pgPort := pg.DSN, pg.Port

	// Force a deterministic config regardless of the developer's shell. PORT
	// and CORS_ORIGIN are the only knobs the launcher exposes.
	setDefault("PORT", "8080")
	setDefault("CORS_ORIGIN", "http://localhost:3000")
	for k, v := range map[string]string{
		"DATABASE_URL":           dsn,
		"LOG_LEVEL":              "warn",
		"SECURE_COOKIES":         "false",
		"CALENDAR_ACCESS_SECRET": strings.Repeat("e", 32),
		"E2E_TEST_MODE":          "true",
		"COOKIE_DOMAIN":          "",
		// Email sending is intercepted by the in-memory capturing sender that
		// the server installs whenever E2E_TEST_MODE is on, so the Resend
		// credentials below are inert. They exist only to satisfy
		// validateAuthEmail; the magic-link spec retrieves the link via
		// GET /test/magic-link.
		"AUTH_EMAIL_DISABLED":          "false",
		"RESEND_API_KEY":               "e2e-fake-key",
		"MAGIC_LINK_FROM_EMAIL":        "e2e@example.test",
		"GOOGLE_CLIENT_ID":             "",
		"GOOGLE_CLIENT_SECRET":         "",
		"GOOGLE_REDIRECT_URL":          "",
		"GOOGLE_CALENDAR_REDIRECT_URL": "",
		"POSTHOG_KEY":                  "",
		"POSTHOG_HOST":                 "",
		"POSTHOG_PERSONAL_API_KEY":     "",
		// FRONTEND_URL is set by the launcher's caller (Playwright config) so
		// magic-link callbacks land back on the right port.
		"FRONTEND_URL": frontendURL(),
	} {
		_ = os.Setenv(k, v)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := waitDB(db, 30*time.Second); err != nil {
		return fmt.Errorf("postgres not ready: %w", err)
	}

	if err := store.Migrate(db, logger); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Loopback only: the e2e backend is never reached from another host, and
	// binding all interfaces makes Windows Firewall prompt on every `go run`
	// (each run is a freshly built binary, so an "Allow" never sticks).
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%s", cfg.Port),
		Handler:           server.New(db, logger, cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("e2e backend listening", "addr", httpSrv.Addr, "db_port", pgPort)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return nil
}

func frontendURL() string {
	if v := os.Getenv("E2E_FRONTEND_URL"); v != "" {
		return v
	}
	return "http://localhost:3100"
}

func setDefault(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}

func waitDB(db *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := db.Ping(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}
