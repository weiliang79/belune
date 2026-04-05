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

// BuildCloneURL constructs an authenticated HTTPS clone URL based on the git provider.
func BuildCloneURL(provider, token, username, repoURL string) string {
	if !strings.HasPrefix(repoURL, "https://") || token == "" {
		return repoURL
	}
	switch provider {
	case "gitlab":
		return strings.Replace(repoURL, "https://", "https://oauth2:"+token+"@", 1)
	case "bitbucket":
		if username == "" {
			username = "x-token-auth"
		}
		return strings.Replace(repoURL, "https://", "https://"+username+":"+token+"@", 1)
	default: // github, generic
		return strings.Replace(repoURL, "https://", "https://"+token+"@", 1)
	}
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
