// Package scripttest runs the real scripts/belune-update.sh (the in-image
// updater), scripts/update.sh (its host launcher) and scripts/backup.sh
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
	// curl -o <dest>: create the file so the staged-fetch loop succeeds. When
	// STAGED_UPDATE is set, the staged scripts/update.sh is that file instead,
	// which is how a test makes the swap replace the running script with
	// different bytes.
	write(t, filepath.Join(dir, "curl"), `#!/bin/sh
url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) dest="$2"; shift ;;
    -*) ;;
    *) url="$1" ;;
  esac
  shift
done
if [ -n "$dest" ]; then
  case "$url" in
    */scripts/update.sh) if [ -n "$STAGED_UPDATE" ]; then cp "$STAGED_UPDATE" "$dest"; else : > "$dest"; fi ;;
    *) : > "$dest" ;;
  esac
fi
exit 0
`, 0o755)
	// DOCKER_LOG, when set, records one line of argv per invocation. When
	// STAGED_LAUNCHER is set, `docker run` overwrites INSTALLED_LAUNCHER with it,
	// which is what the real updater does to scripts/update.sh while the
	// operator's launcher is still waiting on it.
	write(t, filepath.Join(dir, "docker"), `#!/bin/sh
[ -n "$DOCKER_LOG" ] && echo "$*" >> "$DOCKER_LOG"
if [ "$1" = run ] && [ -n "$STAGED_LAUNCHER" ]; then cp "$STAGED_LAUNCHER" "$INSTALLED_LAUNCHER"; fi
[ "$1" = pull ] && [ -n "$DOCKER_PULL_FAIL" ] && exit 1
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
	return runEnv(t, script, install, nil, args...)
}

func runEnv(t *testing.T, script string, install string, extraEnv []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = append(append(os.Environ(), extraEnv...), "BELUNE_DIR="+install, "PATH="+fakeBin(t)+":"+os.Getenv("PATH"))
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

	out, code := run(t, scriptPath(t, "belune-update.sh"), install, "v0.1.11")

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

	out, code := run(t, scriptPath(t, "belune-update.sh"), install, "v0.1.11")

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
			out, _ := run(t, scriptPath(t, "belune-update.sh"), install, "v0.1.11")
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

// The updater lives in the image now, so the swap loop no longer replaces the
// script that is running it — but it still replaces scripts/update.sh, the
// launcher, and an operator running that on the host has a bash waiting on it
// while the updater (inside `docker run`) overwrites it. Bash reads a script by
// byte offset, so an overwrite that changes the length of anything BEFORE that
// point makes it resume mid-content and run garbage (v0.1.11-rc1 executed a
// box-drawing banner as a command). Identical bytes hide this, which is how it
// stayed invisible for three releases. So this installs the real launcher,
// stages a LONGER copy, and has the fake `docker run` swap it in mid-run.
func TestLauncher_SurvivesBeingOverwrittenWhileDockerRunIsInFlight(t *testing.T) {
	real, err := os.ReadFile(scriptPath(t, "update.sh"))
	require.NoError(t, err)

	install := newInstall(t)
	installed := filepath.Join(install, "scripts", "update.sh")
	write(t, installed, string(real), 0o755)

	staged := filepath.Join(t.TempDir(), "update.sh")
	padding := "\n# " + strings.Repeat("padding so the next release's update.sh has a different length ", 8) + "\n"
	stagedBody := strings.Replace(string(real), "\n", padding+"\n", 1)
	require.NotEqual(t, len(real), len(stagedBody))
	write(t, staged, stagedBody, 0o755)

	out, code := runEnv(t, installed, install,
		[]string{"STAGED_LAUNCHER=" + staged, "INSTALLED_LAUNCHER=" + installed}, "v0.1.11")

	require.Equal(t, 0, code, out)
	assert.NotContains(t, out, "command not found")
	swapped, err := os.ReadFile(installed)
	require.NoError(t, err)
	assert.Equal(t, stagedBody, string(swapped), "the swap must actually have replaced the running launcher")
}

// The launcher is the host-side half of a contract old code can never repair:
// pull the target, run ITS /usr/local/bin/belune-update with the version as the
// sole argument. This pins that shape, including the cleared Compose labels
// (without which `docker compose up -d` may remove the helper mid-update) and
// the v-stripped image tag.
func TestLauncher_PullsTargetThenRunsItsUpdater(t *testing.T) {
	install := newInstall(t)
	log := filepath.Join(t.TempDir(), "docker.log")

	out, code := runEnv(t, scriptPath(t, "update.sh"), install, []string{"DOCKER_LOG=" + log}, "v0.1.11")
	require.Equal(t, 0, code, out)

	raw, err := os.ReadFile(log)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 2, string(raw))
	assert.Equal(t, "pull ghcr.io/weiliang79/belune:0.1.11", lines[0])

	run := lines[1]
	assert.True(t, strings.HasPrefix(run, "run "), run)
	assert.Contains(t, run, "--entrypoint /usr/local/bin/belune-update")
	assert.True(t, strings.HasSuffix(run, " ghcr.io/weiliang79/belune:0.1.11 0.1.11"), run)
	for _, want := range []string{
		"--user 0:0", "--network host", "-e BELUNE_DIR=" + install,
		"-v " + install + ":" + install, "-v /var/run/docker.sock:/var/run/docker.sock",
		"--label belune-helper=true", "--label belune-update=true",
		"--label com.docker.compose.project= ", "--label com.docker.compose.service= ",
	} {
		assert.Contains(t, run, want)
	}
	assert.NotContains(t, run, "scripts/", "the updater must not be run from the bind-mounted install dir")
}

func TestLauncher_FailedPullAbortsBeforeRunning(t *testing.T) {
	install := newInstall(t)
	log := filepath.Join(t.TempDir(), "docker.log")
	out, code := runEnv(t, scriptPath(t, "update.sh"), install,
		[]string{"DOCKER_LOG=" + log, "DOCKER_PULL_FAIL=1"}, "v9.9.9")

	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "Could not pull")
	raw, _ := os.ReadFile(log)
	assert.NotContains(t, string(raw), "run ", "nothing may run after a failed pull")
}

// The updater swaps scripts/update.sh (the launcher) from the target release.
// With the updater no longer overwriting itself, this just proves that swap
// still happens and that the launcher is not left non-executable.
func TestUpdate_SwapsInTheLauncher(t *testing.T) {
	install := newInstall(t)
	write(t, filepath.Join(install, "scripts", "backup.sh"), "#!/bin/bash\nexit 0\n", 0o755)
	staged := filepath.Join(t.TempDir(), "update.sh")
	write(t, staged, "#!/bin/bash\n# staged launcher\n", 0o755)

	tmp := t.TempDir()
	out, code := runEnv(t, scriptPath(t, "belune-update.sh"), install, []string{"STAGED_UPDATE=" + staged, "TMPDIR=" + tmp}, "v0.1.11")

	require.Equal(t, 0, code, out)
	assert.Contains(t, out, "Updated 0.1.10")
	got, err := os.ReadFile(filepath.Join(install, "scripts", "update.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "staged launcher")
	fi, err := os.Stat(filepath.Join(install, "scripts", "update.sh"))
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&0o111, "launcher must stay executable")
	assertNoStagingLeft(t, tmp)
}

func TestUpdate_RequiresAVersion(t *testing.T) {
	out, code := run(t, scriptPath(t, "belune-update.sh"), newInstall(t))
	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "No target version")
}

// A failure after the pin moved but before the restart used to leave .env and
// the compose file on the new version while the containers still ran the old
// image, so the abort's "nothing has changed" was false. Here `infra` is a
// regular file, so the swap loop's mkdir for infra/caddy fails AFTER
// docker-compose.yml has already been replaced.
func TestUpdate_FailureBeforeRestartRestoresPinAndInfra(t *testing.T) {
	install := newInstall(t)
	write(t, filepath.Join(install, "scripts", "backup.sh"), "#!/bin/bash\nexit 0\n", 0o755)
	write(t, filepath.Join(install, "infra"), "not a directory\n", 0o644)
	write(t, filepath.Join(install, "docker-compose.yml"), "services: {old: {}}\n", 0o644)
	envBefore, err := os.ReadFile(filepath.Join(install, ".env"))
	require.NoError(t, err)

	tmp := t.TempDir()
	out, code := runEnv(t, scriptPath(t, "belune-update.sh"), install, []string{"TMPDIR=" + tmp}, "v0.1.11")

	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "restoring the previous pin")
	envAfter, _ := os.ReadFile(filepath.Join(install, ".env"))
	assert.Equal(t, string(envBefore), string(envAfter), "the pin must be put back")
	compose, _ := os.ReadFile(filepath.Join(install, "docker-compose.yml"))
	assert.Equal(t, "services: {old: {}}\n", string(compose), "the swapped compose file must be put back")
	assertNoStagingLeft(t, tmp)
}

// assertNoStagingLeft catches a second EXIT trap replacing the one that removes
// the staging directory (bash keeps one handler per signal): the leak would
// otherwise only show up as disk creep, worst on failed runs.
func assertNoStagingLeft(t *testing.T, tmp string) {
	t.Helper()
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries, "update.sh must clean up its staging directory")
}
