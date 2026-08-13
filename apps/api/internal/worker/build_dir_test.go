package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBuildDir(t *testing.T) {
	tmpDir := t.TempDir()
	sub := filepath.Join(tmpDir, "apps", "web")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("empty root directory builds from the clone root", func(t *testing.T) {
		got, err := resolveBuildDir(tmpDir, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tmpDir {
			t.Errorf("got %q, want %q", got, tmpDir)
		}
	})

	t.Run("resolves an existing subdirectory", func(t *testing.T) {
		got, err := resolveBuildDir(tmpDir, "apps/web")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != sub {
			t.Errorf("got %q, want %q", got, sub)
		}
	})

	t.Run("missing subdirectory fails deployment rather than silently building the root", func(t *testing.T) {
		if _, err := resolveBuildDir(tmpDir, "does/not/exist"); err == nil {
			t.Error("expected an error for a subdirectory that does not exist")
		}
	})

	t.Run("traversal segment stays contained within tmpDir even if it slipped past the handler's own validation", func(t *testing.T) {
		if _, err := resolveBuildDir(tmpDir, "../../etc"); err == nil {
			t.Error("expected an error for a traversal path")
		}
	})
}
