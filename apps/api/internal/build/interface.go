package build

import "context"

// BuildOptions contains configuration for a build.
type BuildOptions struct {
	SourceDir      string            // Path to cloned source code
	ImageTag       string            // Target image tag
	DockerfilePath string            // Dockerfile path (for dockerfile builder)
	BuilderImage   string            // CNB builder image (for buildpacks builder)
	Buildpacks     []string          // Ordered list of buildpack URIs (for buildpacks builder)
	Env            map[string]string // Build-time environment variables
	BuildType      string            // Preferred builder: "dockerfile", "buildpacks", "nixpacks" (empty = auto)
}

// BuildResult contains the result of a build.
type BuildResult struct {
	ImageID  string
	ImageTag string
	Logs     string
}

// Builder is the interface that all build strategies implement.
type Builder interface {
	Name() string
	Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
	CanBuild(ctx context.Context, sourceDir string) bool
}
