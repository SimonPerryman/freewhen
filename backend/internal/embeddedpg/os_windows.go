package embeddedpg

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// lockFile takes a non-blocking exclusive lock on f. The OS drops it when the
// handle is closed or the process dies.
func lockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, ol)
}

func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied means it exists but belongs to someone else.
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	const stillActive = 259
	return code == stillActive
}

var (
	jobOnce sync.Once
	jobErr  error
)

// killChildrenOnExit puts this process in a Job Object with
// KILL_ON_JOB_CLOSE. Children (pg_ctl, and the postmaster it detaches) inherit
// the job, so when this process dies — however it dies — the kernel closes
// the last job handle and terminates them too.
func killChildrenOnExit() error {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = err
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			_ = windows.CloseHandle(h)
			jobErr = err
			return
		}
		if err := windows.AssignProcessToJobObject(h, windows.CurrentProcess()); err != nil {
			_ = windows.CloseHandle(h)
			jobErr = err
			return
		}
		// h is deliberately never closed: closing the last handle is what
		// kills the children, and that must happen only when we exit.
	})
	return jobErr
}
