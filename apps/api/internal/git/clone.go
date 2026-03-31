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
// If token is non-empty and repoURL is an HTTPS URL, the token is embedded
// as the username for authentication (e.g. GitHub PAT, GitLab token).
func Clone(ctx context.Context, repoURL, destDir, branch, token string) (*CloneResult, error) {
	cloneURL := repoURL
	if token != "" && strings.HasPrefix(repoURL, "https://") {
		cloneURL = strings.Replace(repoURL, "https://", "https://"+token+"@", 1)
	}

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, destDir)

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
