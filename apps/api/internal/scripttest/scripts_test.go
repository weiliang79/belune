// Package scripttest runs the real scripts/update.sh and scripts/backup.sh
// against fake docker/curl/flock binaries. It proves the control flow (exit
// codes, prompts, what is fatal) — NOT Docker, Postgres or S3 behaviour, which
// only a real stack exercises.
package scripttest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "scripts", name)
	_, err := os.Stat(p)
	require.NoError(t, err)
	return p
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
}

// fakeBin installs stand-ins for the external commands the scripts call.
func fakeBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "flock"), "#!/bin/sh\nexit 0\n", 0o755)
	// curl -o <dest>: create the file so the staged-fetch loop succeeds.
	write(t, filepath.Join(dir, "curl"), `#!/bin/sh
while [ $# -gt 0 ]; do
  if [ "$1" = "-o" ]; then : > "$2"; fi
  shift
done
exit 0
`, 0o755)
	write(t, filepath.Join(dir, "docker"), `#!/bin/sh
case "$1 $2 $3" in
  "compose ps -q") [ "$4" = "postgres" ] && echo fakepg; exit 0 ;;
esac
case "$1" in
  pull) exit 0 ;;
  exec) echo "-- fake dump"; exit 0 ;;
esac
exit 0
`, 0o755)
	return dir
}

func newInstall(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "docker-compose.yml"), "services: {}\n", 0o644)
	write(t, filepath.Join(dir, ".env"), "BELUNE_IMAGE=ghcr.io/weiliang79/belune:0.1.10\nJWT_SECRET=x\n", 0o644)
	return dir
}

func run(t *testing.T, script string, install string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), "BELUNE_DIR="+install, "PATH="+fakeBin(t)+":"+os.Getenv("PATH"))
	cmd.Stdin = nil // /dev/null: the dashboard's detached helper has no stdin
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		require.NoError(t, err)
	}
	return out.String(), code
}

func TestUpdate_FailedBackupWithNoStdinAbortsWithRealCause(t *testing.T) {
	install := newInstall(t)
	write(t, filepath.Join(install, "scripts", "backup.sh"), "#!/bin/bash\necho 'backup boom' >&2\nexit 1\n", 0o755)
	before, _ := os.ReadFile(filepath.Join(install, ".env"))

	out, code := run(t, scriptPath(t, "update.sh"), install, "v0.1.11")

	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "pre-update backup failed")
	assert.Contains(t, out, "no terminal")
	assert.Contains(t, out, "scripts/update.sh")
	assert.NotContains(t, out, "Re-run update.sh")
	after, _ := os.ReadFile(filepath.Join(install, ".env"))
	assert.Equal(t, string(before), string(after), "an aborted update must not move the pin")
}

func TestUpdate_MissingBackupScriptWithNoStdinAbortsWithOwnMessage(t *testing.T) {
	install := newInstall(t)

	out, code := run(t, scriptPath(t, "update.sh"), install, "v0.1.11")

	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "scripts/backup.sh was not found")
	assert.Contains(t, out, "no terminal")
}

func TestUpdate_PassesLocalOnlyFlagOnlyWhenBackupScriptSupportsIt(t *testing.T) {
	// A backup.sh that records its argv and fails: the flag must be present when
	// the script mentions it, absent (it would be read as the output dir) when not.
	for name, tc := range map[string]struct {
		body string
		want bool
	}{
		"supports": {"#!/bin/bash\n# --local-only-ok\necho \"ARGS:$*\"\nexit 1\n", true},
		"legacy":   {"#!/bin/bash\necho \"ARGS:$*\"\nexit 1\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			install := newInstall(t)
			write(t, filepath.Join(install, "scripts", "backup.sh"), tc.body, 0o755)
			out, _ := run(t, scriptPath(t, "update.sh"), install, "v0.1.11")
			assert.Equal(t, tc.want, strings.Contains(out, "ARGS:--local-only-ok"), out)
		})
	}
}

// backupInstall is an install whose remote storage is enabled and whose upload
// helper always fails — the general failure class (endpoint down, rotated
// credentials), not just the missing-binary case.
func backupInstall(t *testing.T, withBinary bool) string {
	install := newInstall(t)
	write(t, filepath.Join(install, "backup-remote.env"), "BACKUP_REMOTE_ENABLED=true\n", 0o644)
	if withBinary {
		write(t, filepath.Join(install, "bin", "belune-backup-upload"), "#!/bin/sh\necho 'dial tcp: connection refused' >&2\nexit 1\n", 0o755)
	}
	return install
}

func TestBackup_RemoteUploadFailureIsFatalByDefault(t *testing.T) {
	for name, withBinary := range map[string]bool{"upload fails": true, "binary missing": false} {
		t.Run(name, func(t *testing.T) {
			out, code := run(t, scriptPath(t, "backup.sh"), backupInstall(t, withBinary))
			assert.NotEqual(t, 0, code, out)
			assert.NotContains(t, out, "Re-run update.sh")
		})
	}
}

func TestBackup_LocalOnlyOKTurnsUploadFailureIntoWarning(t *testing.T) {
	for name, withBinary := range map[string]bool{"upload fails": true, "binary missing": false} {
		t.Run(name, func(t *testing.T) {
			install := backupInstall(t, withBinary)
			out, code := run(t, scriptPath(t, "backup.sh"), install, "--local-only-ok")
			require.Equal(t, 0, code, out)
			assert.Contains(t, out, "[warn]")
			assert.Contains(t, out, "local archive")
			archives, _ := filepath.Glob(filepath.Join(install, "backups", "belune-backup-*.tar.gz"))
			assert.Len(t, archives, 1, "the local archive must exist")
		})
	}
}
