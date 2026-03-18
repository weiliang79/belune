package build

import (
	"context"
	"fmt"
)

// Chain runs through builders in priority order until one succeeds.
type Chain struct {
	builders []Builder
}

func NewChain(builders ...Builder) *Chain {
	return &Chain{builders: builders}
}

// Build tries each builder in order. If BuildType is set, only the matching
// builder is used. Otherwise, builders are tried in order until one succeeds.
func (c *Chain) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	// If a specific build type is requested, use only that builder
	if opts.BuildType != "" {
		for _, b := range c.builders {
			if b.Name() == opts.BuildType {
				if !b.CanBuild(ctx, opts.SourceDir) {
					return nil, &BuildError{Message: fmt.Sprintf("builder %q is not available", opts.BuildType)}
				}
				return b.Build(ctx, opts)
			}
		}
		return nil, &BuildError{Message: fmt.Sprintf("unknown builder: %q", opts.BuildType)}
	}

	// Auto-detect: try each builder in order
	for _, b := range c.builders {
		if b.CanBuild(ctx, opts.SourceDir) {
			result, err := b.Build(ctx, opts)
			if err == nil {
				return result, nil
			}
			// Fall through to next builder on failure
		}
	}
	return nil, ErrNoBuildersAvailable
}

var ErrNoBuildersAvailable = &BuildError{Message: "no suitable builder found"}

type BuildError struct {
	Message string
}

func (e *BuildError) Error() string {
	return e.Message
}
