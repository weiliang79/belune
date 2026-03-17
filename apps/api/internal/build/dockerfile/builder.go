package dockerfile

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "dockerfile" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	_, err := os.Stat(filepath.Join(sourceDir, "Dockerfile"))
	return err == nil
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	// TODO: Build via Docker BuildKit
	return &build.BuildResult{ImageTag: opts.ImageTag}, nil
}
