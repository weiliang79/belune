package railpack

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ungweiliang/selfhost-paas/internal/build"
)

type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Name() string { return "railpack" }

func buildkitHost() string {
	if h := os.Getenv("BUILDKIT_HOST"); h != "" {
		return h
	}
	return "tcp://localhost:1234"
}

func (b *Builder) CanBuild(ctx context.Context, sourceDir string) bool {
	if _, err := exec.LookPath("railpack"); err != nil {
		slog.Debug("railpack CLI not found, skipping railpack builder")
		return false
	}
	return true
}

// CheckBuildKit verifies that the BuildKit daemon is reachable via TCP.
func CheckBuildKit() error {
	host := buildkitHost()
	u, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("invalid BUILDKIT_HOST %q: %w", host, err)
	}

	addr := u.Host
	if addr == "" {
		addr = u.Path
	}

	conn, err := net.DialTimeout(u.Scheme, addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("BuildKit daemon is not reachable at %s — ensure BuildKit is running (see infra/docker-compose.yml): %w", host, err)
	}
	conn.Close()
	return nil
}

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) (*build.BuildResult, error) {
	if err := CheckBuildKit(); err != nil {
		return nil, err
	}

	args := []string{
		"build", opts.SourceDir,
		"--name", opts.ImageTag,
	}

	for k, v := range opts.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	slog.Info("running railpack build", "image", opts.ImageTag, "source", opts.SourceDir)
	cmd := exec.CommandContext(ctx, "railpack", args...)
	cmd.Env = append(os.Environ(), "BUILDKIT_HOST="+buildkitHost())

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
