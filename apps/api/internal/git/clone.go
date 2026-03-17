package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CloneResult contains the result of a git clone operation.
type CloneResult struct {
	CommitSHA string
}

// Clone clones a git repository to the specified directory.
func Clone(ctx context.Context, repoURL, destDir, branch string) (*CloneResult, error) {
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, destDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(output)), err)
	}

	// Get the commit SHA
	shaCmd := exec.CommandContext(ctx, "git", "-C", destDir, "rev-parse", "HEAD")
	shaOutput, err := shaCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}

	return &CloneResult{
		CommitSHA: strings.TrimSpace(string(shaOutput)),
	}, nil
}
