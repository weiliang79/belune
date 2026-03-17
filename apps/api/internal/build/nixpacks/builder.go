package nixpacks

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "nixpacks" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	if _, err := exec.LookPath("nixpacks"); err != nil {
		slog.Debug("nixpacks CLI not found, skipping nixpacks builder")
		return false
	}
	return true
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	args := []string{
		"build", opts.SourceDir,
		"--name", opts.ImageTag,
	}

	for k, v := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	slog.Info("running nixpacks build", "image", opts.ImageTag, "source", opts.SourceDir)
	cmd := exec.CommandContext(ctx, "nixpacks", args...)
	output, err := cmd.CombinedOutput()
	logs := string(output)

	if err != nil {
		return nil, fmt.Errorf("nixpacks build failed: %w\nOutput:\n%s", err, logs)
	}

	return &build.BuildResult{
		ImageTag: opts.ImageTag,
		Logs:     logs,
	}, nil
}
