package railpack

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "railpack" }

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	if _, err := exec.LookPath("railpack"); err != nil {
		slog.Debug("railpack CLI not found, skipping railpack builder")
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

	slog.Info("running railpack build", "image", opts.ImageTag, "source", opts.SourceDir)
	cmd := exec.CommandContext(ctx, "railpack", args...)

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
		return nil, fmt.Errorf("railpack build failed: %w\nOutput:\n%s", err, logs)
	}

	return &build.BuildResult{
		ImageTag: opts.ImageTag,
		Logs:     logs,
	}, nil
}
