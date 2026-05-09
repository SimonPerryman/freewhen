//go:build integration

// Package testdb provides a shared Postgres instance for integration tests.
//
// If TEST_DATABASE_URL is set, Start returns it directly. Otherwise an
// embedded Postgres binary is downloaded (cached under ~/.embedded-postgres-go/)
// and started in-process on a free port, once per test binary.
package testdb

import (
	"database/sql"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const dbName = "freewhen_test"

var (
	once      sync.Once
	sharedDSN string
	startErr  error
)

// Start returns a DSN for a Postgres instance ready to run migrations against.
// The instance is shared across all tests in the binary; callers rely on
// per-test table truncation for isolation.
func Start(t *testing.T) string {
	t.Helper()
	once.Do(initShared)
	if startErr != nil {
		t.Fatalf("testdb: %v", startErr)
	}
	return sharedDSN
}

func initShared() {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		sharedDSN = dsn
		return
	}

	port, err := freePort()
	if err != nil {
		startErr = fmt.Errorf("pick port: %w", err)
		return
	}

	runtimeDir, err := os.MkdirTemp("", "freewhen-epg-*")
	if err != nil {
		startErr = fmt.Errorf("mkdir runtime: %w", err)
		return
	}

	cfg := embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		Database(dbName).
		Username("postgres").
		Password("postgres").
		RuntimePath(runtimeDir).
		DataPath(runtimeDir + "/data").
		Logger(io.Discard).
		StartTimeout(60 * time.Second)

	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		startErr = fmt.Errorf("start embedded postgres: %w", err)
		return
	}

	sharedDSN = fmt.Sprintf("postgres://postgres:postgres@localhost:%d/%s?sslmode=disable", port, dbName)

	if err := waitReady(sharedDSN); err != nil {
		_ = pg.Stop()
		startErr = fmt.Errorf("wait ready: %w", err)
		return
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := db.Ping(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("postgres not ready: %w", lastErr)
}
