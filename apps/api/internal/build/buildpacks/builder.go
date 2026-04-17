package buildpacks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/ungweiliang/selfhost-paas/internal/build"
	"github.com/ungweiliang/selfhost-paas/internal/naming"
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

	// Wire persistent build+launch caches keyed by application ID so each app
	// reuses its own layer history across deploys. Without these flags pack
	// derives an auto-generated volume name from the image digest, which
	// churns every time the image tag changes (we tag each deploy uniquely).
	//
	// Pre-create the volumes so they carry the paas-cache label — auto-created
	// volumes would not be labelled and the periodic cleanup worker would
	// wipe them via PruneVolumes.
	if opts.ApplicationID != "" {
		buildVol := naming.CNBCacheVolumeName(opts.ApplicationID)
		launchVol := naming.CNBLaunchCacheVolumeName(opts.ApplicationID)
		if b.runtime != nil {
			if err := b.runtime.CreateCacheVolume(ctx, buildVol); err != nil {
				slog.Warn("ensure cnb build cache volume", "volume", buildVol, "error", err)
			}
			if err := b.runtime.CreateCacheVolume(ctx, launchVol); err != nil {
				slog.Warn("ensure cnb launch cache volume", "volume", launchVol, "error", err)
			}
		}
		args = append(args,
			"--cache", fmt.Sprintf("type=build;format=volume;name=%s", buildVol),
			"--cache", fmt.Sprintf("type=launch;format=volume;name=%s", launchVol),
		)
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
