//go:build !windows

package embeddedpg

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive flock on f. The OS drops it when the
// descriptor is closed or the process dies (Go opens files O_CLOEXEC, so the
// postmaster never inherits it).
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// killChildrenOnExit is a no-op off Windows: pg_ctl setsid()s the postmaster
// away from us, so there is no cheap equivalent of a job object. ReapStale
// cleans up after an owner that died without calling Stop.
func killChildrenOnExit() error { return nil }
