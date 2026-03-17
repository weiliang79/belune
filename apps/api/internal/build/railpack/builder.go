package railpack

import (
	"context"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "railpack" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	// TODO: Check if Railpack supports this source
	return false
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	// TODO: Run railpack CLI (behind FEATURE_RAILPACK flag)
	return &build.BuildResult{ImageTag: opts.ImageTag}, nil
}
