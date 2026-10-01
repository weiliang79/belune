package worker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A HELD lock and an UNOPENABLE lock are different operator problems: one is
// "wait", the other is "this will never work until you fix the file". Reporting
// both as "already in progress" sent a real operator hunting for a run that did
// not exist (found 2026-10-01 on a live install, where the pre-update backup
// left .lock owned by root and the non-root worker could not open it).
func TestBackupLockFailure_DistinguishesPermissionFromHeld(t *testing.T) {
	held := BackupLockFailure(errors.New("lock held by another backup run: resource temporarily unavailable"), "/x/.lock")
	assert.Equal(t, "a control-plane backup is already in progress", held.Error())

	denied := BackupLockFailure(&fs.PathError{Op: "open", Path: "/x/.lock", Err: syscall.EACCES}, "/x/.lock")
	assert.Contains(t, denied.Error(), "/x/.lock", "name the file, or nobody can fix it")
	assert.Contains(t, denied.Error(), "no backup is actually running")
	assert.NotContains(t, denied.Error(), "already in progress",
		"a permission error must not claim a run is in progress")
}

// The real path end to end: a lock file this process cannot open must surface as
// the permission message, not as a phantom concurrent run.
func TestAcquireFileLock_UnopenableFileIsAPermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can open anything; this asserts the non-root worker's experience")
	}
	path := filepath.Join(t.TempDir(), ".lock")
	require.NoError(t, os.WriteFile(path, nil, 0o000))

	_, err := acquireFileLock(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrPermission, "must be classifiable, not an opaque string")
	assert.NotContains(t, BackupLockFailure(err, path).Error(), "already in progress")
}
