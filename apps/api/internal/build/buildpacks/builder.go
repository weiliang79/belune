package buildpacks

import (
	"context"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
)

// Builder runs Cloud Native Buildpacks (CNB) via `pack build`.
type Builder struct {
	runtime             runtime.ContainerRuntime
	defaultBuilderImage string
}

func New(rt runtime.ContainerRuntime) *Builder {
	return &Builder{
		runtime:             rt,
		defaultBuilderImage: DefaultBuilderImage,
	}
}

func (b *Builder) Name() string { return "buildpacks" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	// CNB can attempt to build most source directories
	return true
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	builderImage := b.defaultBuilderImage
	if opts.BuilderImage != "" {
		builderImage = opts.BuilderImage
	}

	// TODO: Run `pack build` with:
	//   --builder <builderImage>
	//   --buildpack <bp1> --buildpack <bp2> ... (if opts.Buildpacks is set)
	//   --tag <opts.ImageTag>
	//   --path <opts.SourceDir>
	_ = builderImage

	return &build.BuildResult{ImageTag: opts.ImageTag}, nil
}
