//go:build integration

package embeddedpg

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func backdate(t *testing.T, dir string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestStopRemovesEverything(t *testing.T) {
	inst, err := Start(PrefixTest, "reap_test")
	if err != nil {
		t.Fatal(err)
	}
	dir := inst.dir
	pid := postmasterPID(dir)
	if pid == 0 || !processAlive(pid) {
		t.Fatalf("postmaster not running (pid %d)", pid)
	}
	if err := inst.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("runtime dir still exists: %v", err)
	}
	if processAlive(pid) {
		t.Fatalf("postmaster %d still alive after Stop", pid)
	}
}

func TestReapStale(t *testing.T) {
	inst, err := Start(PrefixTest, "reap_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.Stop() })
	dir := inst.dir
	pid := postmasterPID(dir)
	backdate(t, dir)

	// Owner alive (lock held): must be left alone.
	ReapStale(io.Discard)
	if _, err := os.Stat(filepath.Join(dir, "data")); err != nil {
		t.Fatalf("live instance was reaped: %v", err)
	}
	if !processAlive(pid) {
		t.Fatal("live postmaster was stopped")
	}

	// Simulate the owner dying without Stop: drop the lock, keep Postgres up.
	_ = inst.lock.Close()
	inst.lock = nil
	backdate(t, dir)

	if n := ReapStale(io.Discard); n < 1 {
		t.Fatalf("ReapStale reaped %d instances, want >= 1", n)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("stale dir still exists: %v", err)
	}
	if processAlive(pid) {
		t.Fatalf("stale postmaster %d still alive", pid)
	}
	inst.pg, inst.dir = nil, ""
}
