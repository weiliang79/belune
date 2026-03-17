package nixpacks

import (
	"context"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "nixpacks" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	// Nixpacks can build almost anything
	return true
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	// TODO: Run `nixpacks build` CLI
	return &build.BuildResult{ImageTag: opts.ImageTag}, nil
}
