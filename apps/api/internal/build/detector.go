package build

import (
	"context"
	"os"
	"path/filepath"
)

// Detect returns the best builder for the given source directory.
// Priority: Dockerfile → CNB Buildpacks → Railpack
func Detect(ctx context.Context, sourceDir string, builders []Builder) Builder {
	// Check for Dockerfile first
	dockerfilePath := filepath.Join(sourceDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		for _, b := range builders {
			if b.Name() == "dockerfile" {
				return b
			}
		}
	}

	// Try each builder in order
	for _, b := range builders {
		if b.CanBuild(ctx, sourceDir) {
			return b
		}
	}

	return nil
}
