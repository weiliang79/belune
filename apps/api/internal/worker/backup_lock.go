package worker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// fileLock is an advisory, exclusive lock held via flock(2) on a fixed path.
// scripts/backup.sh (host CLI) takes the same lock with `flock -n` against the
// same path (bind-mounted from the same directory), so the worker and the CLI
// can never write two control-plane backup archives at once.
type fileLock struct {
	f *os.File
}

// acquireFileLock takes a non-blocking exclusive lock on path, creating the
// file if needed. Returns an error immediately if the lock is already held
// (never blocks) — callers should surface that as "a backup is already in
// progress" rather than retry within the same task.
//
// ⚠️ It can also fail WITHOUT the lock being held, and the two must not be
// reported the same way: the lock is shared with scripts/backup.sh, which runs
// as root, so a root-created 0644 lock file cannot be opened O_RDWR by the
// non-root uid this process runs as. That fails at the open. Reporting it as
// "already in progress" sends the operator looking for a run that does not
// exist, and the condition is permanent rather than transient — use
// BackupLockFailure to phrase it.
func acquireFileLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lockfile: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock held by another backup run: %w", err)
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}

// BackupLockFailure turns an acquireFileLock error into something an operator
// can act on. A held lock is a transient "wait and retry"; a permission error
// is a stuck file that will fail identically forever until someone fixes it, so
// it must say so and name the path.
func BackupLockFailure(err error, path string) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("cannot open the backup lock at %s — it is not writable by this container, "+
			"which a backup run as root (the host CLI, or the pre-update backup) can cause. "+
			"Fix its ownership on the host and retry; no backup is actually running", path)
	}
	return errors.New("a control-plane backup is already in progress")
}
