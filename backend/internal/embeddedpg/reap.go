package embeddedpg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// minAge protects a dir another process has just created but not yet
	// locked (MkdirTemp and the lock are two steps).
	minAge = 10 * time.Second
	// legacyMaxAge bounds how long a lock-less dir (created before owner
	// locks existed) with a live postmaster is left alone.
	legacyMaxAge = 2 * time.Hour
)

// ReapStale stops and deletes abandoned instances left in os.TempDir() by
// killed test binaries or e2e servers. It never touches an instance whose
// owner is still alive (owner lock held). Progress is written to w.
func ReapStale(w io.Writer) (reaped int) {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() || !hasKnownPrefix(e.Name()) {
			continue
		}
		dir := filepath.Join(os.TempDir(), e.Name())
		ok, why := reapOne(dir)
		switch {
		case ok:
			reaped++
			_, _ = fmt.Fprintf(w, "embeddedpg: reaped stale instance %s (%s)\n", e.Name(), why)
		case why != "":
			_, _ = fmt.Fprintf(w, "embeddedpg: could not reap %s: %s\n", e.Name(), why)
		}
	}
	return reaped
}

func hasKnownPrefix(name string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func reapOne(dir string) (bool, string) {
	info, err := os.Stat(dir)
	if err != nil || time.Since(info.ModTime()) < minAge {
		return false, ""
	}

	why := "owner exited"
	lock, err := os.OpenFile(filepath.Join(dir, lockName), os.O_RDWR, 0)
	switch {
	case err == nil:
		if lockFile(lock) != nil {
			_ = lock.Close()
			return false, "" // owner alive
		}
		// We hold the lock until the dir is gone, so a second reaper
		// running concurrently skips it.
		defer func() { _ = lock.Close() }()
	case os.IsNotExist(err):
		// Pre-lock dir: reap only if its postmaster is dead, or it is old.
		pid := postmasterPID(dir)
		alive := pid > 0 && processAlive(pid)
		if alive && time.Since(info.ModTime()) < legacyMaxAge {
			return false, ""
		}
		why = "legacy dir, postmaster dead"
		if alive {
			why = "legacy dir older than " + legacyMaxAge.String()
		}
	default:
		return false, ""
	}

	if pid := postmasterPID(dir); pid > 0 && processAlive(pid) {
		// Pre-lock dirs kept the binaries at <dir>/bin.
		pgCtl := filepath.Join(dir, runtimeSubdir, "bin", "pg_ctl")
		if _, err := os.Stat(filepath.Dir(pgCtl)); err != nil {
			pgCtl = filepath.Join(dir, "bin", "pg_ctl")
		}
		cmd := exec.Command(pgCtl, "stop", "-m", "immediate", "-w", "-t", "30", "-D", filepath.Join(dir, "data"))
		if out, err := cmd.CombinedOutput(); err != nil && processAlive(pid) {
			return false, fmt.Sprintf("pg_ctl stop failed: %v: %s", err, strings.TrimSpace(string(out)))
		}
		why += ", stopped postmaster"
	}

	// Release our handle before deleting: Windows refuses to remove open files.
	if lock != nil {
		_ = lock.Close()
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, err.Error()
	}
	return true, why
}

// postmasterPID returns the PID recorded in data/postmaster.pid, or 0.
func postmasterPID(dir string) int {
	f, err := os.Open(filepath.Join(dir, "data", "postmaster.pid"))
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s.Text()))
	if err != nil {
		return 0
	}
	return pid
}
