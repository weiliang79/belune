package buildpacks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"

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
	if _, err := exec.LookPath("pack"); err != nil {
		slog.Debug("pack CLI not found, skipping buildpacks builder")
		return false
	}
	return true
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	builderImage := b.defaultBuilderImage
	if opts.BuilderImage != "" {
		builderImage = opts.BuilderImage
	}

	args := []string{
		"build", opts.ImageTag,
		"--builder", builderImage,
		"--path", opts.SourceDir,
		"--trust-builder",
	}

	for _, bp := range opts.Buildpacks {
		args = append(args, "--buildpack", bp)
	}

	for k, v := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	slog.Info("running pack build", "image", opts.ImageTag, "builder", builderImage)
	cmd := exec.CommandContext(ctx, "pack", args...)

	var logBuf strings.Builder
	writers := []io.Writer{&logBuf}
	if opts.LogWriter != nil {
		writers = append(writers, opts.LogWriter)
	}
	multi := io.MultiWriter(writers...)
	cmd.Stdout = multi
	cmd.Stderr = multi

	err := cmd.Run()
	logs := logBuf.String()

	if err != nil {
		return nil, fmt.Errorf("pack build failed: %w\nOutput:\n%s", err, logs)
	}

	return &build.BuildResult{
		ImageTag: opts.ImageTag,
		Logs:     logs,
	}, nil
}
