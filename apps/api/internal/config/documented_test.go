package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exist because the configuration had drifted across four sources
// of truth — config.go, .env.defaults, .env.example and a heredoc inside
// scripts/install.sh — and nothing compared them. The visible cost was
// JWT_EXPIRY_HOURS: 1 in the code and in both env files, 24 in the installer, so
// every production install issued access tokens with a lifetime 24x longer than
// intended and no one could see it.
//
// Splitting the env files does not fix that on its own; more files is more
// surface. What fixes it is something that fails when they disagree.

// repoRoot walks up from this package to the directory holding go.mod's parent
// project (the repository root, two levels above apps/api).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dir, "docs", "configuration.md"),
		"expected the repo root to contain docs/configuration.md")
	return dir
}

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(parts...))
	require.NoError(t, err)
	return string(b)
}

// envReadRe matches both ways the tree resolves a variable. Scanning only
// config.go is not enough and the first run of this test proved it: BUILDKIT_HOST
// is read with os.Getenv inside the railpack builder, so a config-only scraper
// called it undocumented when it was documented, and would have missed a genuine
// omission in any package that reads the environment directly.
var envReadRe = regexp.MustCompile(`(?:getEnv\w*|os\.Getenv)\("([A-Z0-9_]+)"`)

// envVarsReadByCode walks the API source for every variable it resolves.
func envVarsReadByCode(t *testing.T) []string {
	t.Helper()
	apiRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(apiRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Test files reference variables they set up themselves; they are not
			// configuration the product exposes.
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range envReadRe.FindAllStringSubmatch(string(b), -1) {
				seen[m[1]] = true
			}
			return nil
		})
		require.NoError(t, err)
	}

	require.NotEmpty(t, seen, "scraper matched nothing — has the source changed shape?")
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Every variable the code reads has to appear in the reference. A variable that
// exists but is written down nowhere is one nobody can discover.
func TestEveryEnvVarIsDocumented(t *testing.T) {
	doc := readFile(t, repoRoot(t), "docs", "configuration.md")

	var missing []string
	for _, v := range envVarsReadByCode(t) {
		if !strings.Contains(doc, v) {
			missing = append(missing, v)
		}
	}
	assert.Empty(t, missing,
		"these are read by config.go but absent from docs/configuration.md — document them or stop reading them:\n  %s",
		strings.Join(missing, "\n  "))
}

// The seeds may be small — that is the point of a seed — but every line in one
// has to be a variable the code actually reads. A stale name in a seed is worse
// than an absent one: it looks like configuration and does nothing.
func TestSeedFilesOnlyContainRealVars(t *testing.T) {
	root := repoRoot(t)
	known := map[string]bool{}
	for _, v := range envVarsReadByCode(t) {
		known[v] = true
	}
	// Consumed by Compose and the install scripts rather than the Go binary.
	for _, v := range []string{
		"BELUNE_IMAGE", "DOCKER_GID",
		"POSTGRES_USER", "POSTGRES_DB", "POSTGRES_PASSWORD",
	} {
		known[v] = true
	}

	assign := regexp.MustCompile(`(?m)^([A-Z0-9_]+)=`)
	for _, seed := range []string{".env.example", ".env.example.dev"} {
		t.Run(seed, func(t *testing.T) {
			var unknown []string
			for _, m := range assign.FindAllStringSubmatch(readFile(t, root, seed), -1) {
				if !known[m[1]] {
					unknown = append(unknown, m[1])
				}
			}
			assert.Empty(t, unknown,
				"%s sets variables nothing reads:\n  %s", seed, strings.Join(unknown, "\n  "))
		})
	}
}

// The installer writes its own .env from a heredoc, which is a fourth place a
// value can be stated. It is allowed to differ from the code default — that is
// what an installer is for — but only deliberately, so any value it pins has to
// be one the reference also mentions.
func TestInstallerDoesNotInventVariables(t *testing.T) {
	root := repoRoot(t)
	script := readFile(t, root, "scripts", "install.sh")

	// Only the generated heredoc, not the whole script.
	start := strings.Index(script, "cat > .env <<EOF")
	require.Greater(t, start, -1, "could not find the .env heredoc in install.sh")
	end := strings.Index(script[start:], "\nEOF")
	require.Greater(t, end, -1)
	heredoc := script[start : start+end]

	known := map[string]bool{}
	for _, v := range envVarsReadByCode(t) {
		known[v] = true
	}
	for _, v := range []string{"BELUNE_IMAGE", "DOCKER_GID", "POSTGRES_USER", "POSTGRES_DB", "POSTGRES_PASSWORD"} {
		known[v] = true
	}

	assign := regexp.MustCompile(`(?m)^([A-Z0-9_]+)=`)
	var unknown []string
	for _, m := range assign.FindAllStringSubmatch(heredoc, -1) {
		if !known[m[1]] {
			unknown = append(unknown, m[1])
		}
	}
	assert.Empty(t, unknown,
		"install.sh writes variables nothing reads:\n  %s", strings.Join(unknown, "\n  "))
}

// The specific regression that started this: a security-relevant value pinned in
// the installer that contradicted the code default, invisibly.
func TestInstallerDoesNotWeakenTokenLifetime(t *testing.T) {
	script := readFile(t, repoRoot(t), "scripts", "install.sh")
	assert.NotContains(t, script, "JWT_EXPIRY_HOURS=",
		"the installer pins the access-token lifetime again. It was 24 while the code "+
			"default was 1, so every install had tokens valid 24x longer than intended. "+
			"Leave it unset and let the code default apply, or change the code default.")
}
