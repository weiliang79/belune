package worker

import (
	"fmt"
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
