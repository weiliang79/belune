package build

import "context"

// Chain runs through builders in priority order until one succeeds.
type Chain struct {
	builders []Builder
}

func NewChain(builders ...Builder) *Chain {
	return &Chain{builders: builders}
}

// Build tries each builder in order. If a builder reports CanBuild=true,
// it attempts the build. On failure, it falls through to the next builder.
func (c *Chain) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
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
