//go:build linux

package scripttest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// ttyStdin returns the slave end of a fresh pseudo-terminal, with input already
// typed into it, to be used as a child's stdin. The tty's line discipline holds
// the line until the child reads it, so no handshake with the prompt is needed;
// "\x04" is Ctrl-D.
//
// This is the only way to reach the updater's interactive branch honestly:
// "stdin is a terminal" is a property of the file descriptor, and a pipe or a
// file that merely looks interactive proves nothing about the isatty call.
func ttyStdin(t *testing.T, input string) *os.File {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })

	var unlock int32
	var n uint32
	if err := ioctl(master, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != nil {
		t.Skipf("cannot unlock the pty: %v", err)
	}
	if err := ioctl(master, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		t.Skipf("cannot name the pty: %v", err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("cannot open the pty slave: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })

	if _, err := master.WriteString(input); err != nil {
		t.Fatalf("typing into the pty: %v", err)
	}
	return slave
}

func ioctl(f *os.File, req, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, arg); errno != 0 {
		return errno
	}
	return nil
}

// "No terminal" is decided by isatty, not by whether stdin looks like a file or a
// device — /dev/null is a character device, and it is what the dashboard's
// detached helper has. The existing no-stdin tests prove the helper is told there
// is no terminal to ask on; this proves the other half: with a REAL terminal the
// operator is asked, and the answer is honoured.
func TestUpdate_AsksOnATerminal(t *testing.T) {
	for name, tc := range map[string]struct {
		typed    string
		wantCode int
		want     string
	}{
		"yes continues": {"y\n", 0, "Updated 0.1.10"},
		"no aborts":     {"n\n", 1, "Aborted. Nothing has changed."},
	} {
		t.Run(name, func(t *testing.T) {
			install := newInstall(t)
			write(t, filepath.Join(install, "scripts", "backup.sh"), "#!/bin/bash\necho 'backup boom' >&2\nexit 1\n", 0o755)
			before, _ := os.ReadFile(filepath.Join(install, ".env"))

			cmd := exec.Command(updaterBinary(t), "v0.1.11")
			cmd.Stdin = ttyStdin(t, tc.typed)
			out, code := runCmd(t, cmd, install, nil)

			assert.Equal(t, tc.wantCode, code, out)
			assert.Contains(t, out, "Continue updating without a backup? [y/N] ")
			assert.Contains(t, out, tc.want)
			assert.NotContains(t, out, "no terminal", "a real terminal must be asked, not told there is none")
			if tc.wantCode != 0 {
				after, _ := os.ReadFile(filepath.Join(install, ".env"))
				assert.Equal(t, string(before), string(after))
			}
		})
	}
}
