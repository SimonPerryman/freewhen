//go:build integration

// Package testdb provides a shared Postgres instance for integration tests.
//
// If TEST_DATABASE_URL is set, Start returns it directly. Otherwise an
// embedded Postgres (binary cached under ~/.embedded-postgres-go/) is started
// lazily on a free port, once per test binary, and torn down by Main.
//
// Every package that calls Start must route its TestMain through Main:
//
//	func TestMain(m *testing.M) { testdb.Main(m) }
package testdb

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"

	"github.com/freewhen/backend/internal/embeddedpg"
)

const dbName = "freewhen_test"

var (
	mu       sync.Mutex
	inMain   bool
	started  bool
	instance *embeddedpg.Instance
	dsn      string
	startErr error
)

// Main runs the package's tests and then stops the shared instance and
// deletes its temp dir. It also cleans up on SIGINT/SIGTERM. Call it from
// TestMain; it does not return.
func Main(m *testing.M) {
	mu.Lock()
	inMain = true
	mu.Unlock()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		shutdown()
		os.Exit(130)
	}()

	code := m.Run()
	shutdown()
	os.Exit(code)
}

// Start returns a DSN for a Postgres instance ready to run migrations against.
// The instance is shared across all tests in the binary; callers rely on
// per-test table truncation for isolation.
func Start(t *testing.T) string {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if !inMain {
		t.Fatal("testdb: call testdb.Main(m) from this package's TestMain so the embedded Postgres is stopped on exit")
	}
	if !started {
		started = true
		dsn, startErr = start()
	}
	if startErr != nil {
		t.Fatalf("testdb: %v", startErr)
	}
	return dsn
}

func start() (string, error) {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v, nil
	}
	inst, err := embeddedpg.Start(embeddedpg.PrefixTest, dbName)
	if err != nil {
		return "", err
	}
	instance = inst
	return inst.DSN, nil
}

func shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if instance == nil {
		return
	}
	if err := instance.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: %v\n", err)
	}
	instance = nil
}
