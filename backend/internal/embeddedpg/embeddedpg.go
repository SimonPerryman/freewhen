// Package embeddedpg runs a throwaway embedded Postgres for integration tests
// and the e2e server, without leaking processes or temp dirs.
//
// Each instance lives in its own os.TempDir()/<prefix>* dir (the extracted
// binaries plus the data dir, ~150 MB). Leaks are prevented in
// three layers:
//
//  1. Stop() stops Postgres and deletes the runtime dir. Callers defer it.
//  2. On Windows the owning process is put in a Job Object that kills every
//     child — including the daemonised postmaster — when the owner exits for
//     any reason (taskkill /F, test timeout, panic). No-op elsewhere.
//  3. The owner holds an exclusive OS lock on <dir>/owner.lock for the
//     instance's lifetime. Start() first reaps every runtime dir whose lock
//     can be taken (its owner is gone): it stops any surviving postmaster
//     with `pg_ctl stop -m immediate` and deletes the dir.
package embeddedpg

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Runtime dir prefixes. Every prefix used with Start must be listed in
// prefixes so ReapStale can find abandoned instances.
const (
	PrefixTest = "freewhen-epg-"
	PrefixE2E  = "freewhen-e2e-pg-"
)

var prefixes = []string{PrefixTest, PrefixE2E}

// Layout of a runtime dir: <dir>/owner.lock, <dir>/rt/bin/... (extracted
// binaries), <dir>/data (cluster).
const (
	lockName      = "owner.lock"
	runtimeSubdir = "rt"
)

// Instance is a running embedded Postgres.
type Instance struct {
	DSN  string
	Port int

	dir  string
	pg   *embeddedpostgres.EmbeddedPostgres
	lock *os.File
}

// Start reaps abandoned instances, then boots a fresh Postgres with the given
// database (user/password postgres/postgres) on a free loopback port.
func Start(prefix, database string) (*Instance, error) {
	if err := killChildrenOnExit(); err != nil {
		// Not fatal: the reaper still cleans up after us next time.
		fmt.Fprintf(os.Stderr, "embeddedpg: job object setup failed: %v\n", err)
	}
	ReapStale(os.Stderr)

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("pick port: %w", err)
	}

	dir, err := os.MkdirTemp("", prefix+"*")
	if err != nil {
		return nil, fmt.Errorf("mkdir runtime: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err == nil {
		err = lockFile(lock)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("lock runtime dir: %w", err)
	}
	_, _ = fmt.Fprintf(lock, "%d\n", os.Getpid())

	inst := &Instance{
		Port: port,
		DSN:  fmt.Sprintf("postgres://postgres:postgres@localhost:%d/%s?sslmode=disable", port, database),
		dir:  dir,
		lock: lock,
	}

	cfg := embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		Database(database).
		Username("postgres").
		Password("postgres").
		// embedded-postgres wipes RuntimePath on Start, so the lock file
		// must live beside it, not inside it.
		RuntimePath(filepath.Join(dir, runtimeSubdir)).
		DataPath(filepath.Join(dir, "data")).
		Logger(io.Discard).
		StartTimeout(60 * time.Second)
	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		// With a cold ~/.embedded-postgres-go, parallel test binaries all
		// download the archive and race to rename it into place; on Windows
		// the losers fail with "Access is denied". By now the winner's copy
		// is in the cache, so one retry succeeds.
		time.Sleep(time.Second)
		pg = embeddedpostgres.NewDatabase(cfg)
		if err := pg.Start(); err != nil {
			_ = inst.Stop()
			return nil, fmt.Errorf("start embedded postgres: %w", err)
		}
	}
	inst.pg = pg

	if err := waitReady(inst.DSN, 30*time.Second); err != nil {
		_ = inst.Stop()
		return nil, fmt.Errorf("wait ready: %w", err)
	}
	return inst, nil
}

// Stop stops Postgres and removes the runtime dir. Safe to call more than
// once.
func (i *Instance) Stop() error {
	var errs []error
	if i.pg != nil {
		if err := i.pg.Stop(); err != nil && !errors.Is(err, embeddedpostgres.ErrServerNotStarted) {
			errs = append(errs, fmt.Errorf("stop postgres: %w", err))
		}
		i.pg = nil
	}
	if i.lock != nil {
		_ = i.lock.Close() // releases the OS lock
		i.lock = nil
	}
	if i.dir != "" {
		if err := os.RemoveAll(i.dir); err != nil {
			errs = append(errs, fmt.Errorf("remove runtime dir: %w", err))
		}
		i.dir = ""
	}
	return errors.Join(errs...)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(dsn string, timeout time.Duration) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if lastErr = db.Ping(); lastErr == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("postgres not ready: %w", lastErr)
}
