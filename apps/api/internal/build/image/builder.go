package image

import (
	"context"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

// Builder handles pre-built images — just pull and deploy, no build step.
type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "image" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	return false // Only used when explicitly selected
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	// TODO: Pull the pre-built image
	return &build.BuildResult{ImageTag: opts.ImageTag}, nil
}
