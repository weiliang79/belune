package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveBuildDir returns the directory a build should run from: tmpDir (the
// repo clone root) by default, or a subdirectory of it when the application
// has a root directory configured (monorepo support). Everything downstream
// — detection, Railpack, buildpacks, Dockerfile context — then runs from the
// resolved directory automatically, since it becomes the build's SourceDir.
//
// A leading "/" neutralizes ".." segments before joining, and the result is
// checked to still be inside tmpDir as defense in depth against traversal.
func resolveBuildDir(tmpDir, rootDirectory string) (string, error) {
	rd := strings.TrimSpace(rootDirectory)
	if rd == "" {
		return tmpDir, nil
	}
	buildDir := filepath.Join(tmpDir, filepath.Clean("/"+rd))
	if !strings.HasPrefix(buildDir, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid root directory %q", rd)
	}
	if info, err := os.Stat(buildDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("root directory %q not found in repository", rd)
	}
	return buildDir, nil
}
