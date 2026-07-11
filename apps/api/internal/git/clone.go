package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// CloneResult contains the result of a git clone operation.
type CloneResult struct {
	CommitSHA string
}

// ErrUnsafeRepoURL is returned when a repo URL points at a location we will
// not clone from — local filesystem schemes, loopback, link-local, or RFC1918
// addresses. The check is best-effort SSRF mitigation for the git-clone path.
var ErrUnsafeRepoURL = errors.New("repository URL is not allowed")

// allowedSchemes are the only URL schemes we accept for HTTP(S) clones.
// SSH-form URLs (`git@host:path`) are accepted via a separate code path
// because url.Parse does not understand them.
var allowedSchemes = map[string]struct{}{
	"https": {},
	"http":  {}, // intentionally allowed; some self-hosted gitea/gitlab run on http internally
}

// blockedHosts covers obviously-local hostnames that should never appear
// as a clone target. Hostname matching is case-insensitive.
var blockedHosts = map[string]struct{}{
	"localhost":     {},
	"localhost.":    {},
	"ip6-localhost": {},
}

// privateOrLinkLocal lists CIDR ranges we refuse to clone from. Resolved
// addresses for the repo host are checked against these ranges so a
// malicious DNS record cannot redirect us into a private network.
var privateOrLinkLocal []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback v4
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // link-local v4 + cloud metadata services
		"100.64.0.0/10",  // CGNAT / Tailscale
		"::1/128",        // loopback v6
		"fc00::/7",       // ULA v6
		"fe80::/10",      // link-local v6
	}
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			slog.Error("git: bad CIDR in privateOrLinkLocal", "cidr", s, "error", err)
			os.Exit(1)
		}
		privateOrLinkLocal = append(privateOrLinkLocal, n)
	}
}

// validateRepoURL performs scheme + host + resolved-IP checks. It is the
// only gate between an arbitrary user-supplied repo URL and our git binary.
// hostResolver is overrideable so tests can run without DNS.
func validateRepoURL(repoURL string, hostResolver func(string) ([]net.IP, error)) error {
	if repoURL == "" {
		return ErrUnsafeRepoURL
	}

	// SSH-form: `git@host:owner/repo.git`. We accept these but apply the same
	// host check as URL form.
	if isSSHForm(repoURL) {
		host := sshHost(repoURL)
		return checkHost(host, hostResolver)
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("%w: parse: %v", ErrUnsafeRepoURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if _, ok := allowedSchemes[scheme]; !ok {
		// Includes the dangerous ones explicitly: file://, gopher://, ftp://, etc.
		return fmt.Errorf("%w: scheme %q not allowed", ErrUnsafeRepoURL, scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrUnsafeRepoURL)
	}
	host := u.Hostname()
	return checkHost(host, hostResolver)
}

func checkHost(host string, hostResolver func(string) ([]net.IP, error)) error {
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrUnsafeRepoURL)
	}
	hostLower := strings.ToLower(host)
	if _, blocked := blockedHosts[hostLower]; blocked {
		return fmt.Errorf("%w: host %q is loopback", ErrUnsafeRepoURL, host)
	}

	// If the host is already an IP literal, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: address %s is private/link-local", ErrUnsafeRepoURL, ip.String())
		}
		return nil
	}

	// Otherwise resolve DNS and check every returned address. A single
	// blocked address rejects the URL — defence against split-horizon DNS
	// or misconfigured records pointing into the cluster.
	if hostResolver == nil {
		hostResolver = defaultResolver
	}
	ips, err := hostResolver(host)
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %v", ErrUnsafeRepoURL, host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: no addresses for %s", ErrUnsafeRepoURL, host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: %s resolves to %s (private/link-local)", ErrUnsafeRepoURL, host, ip.String())
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	for _, n := range privateOrLinkLocal {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func defaultResolver(host string) ([]net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// isSSHForm detects `git@host:path` URLs. These do not have a scheme and
// url.Parse treats them as relative paths. Recognising them explicitly
// avoids accidentally rejecting legitimate ssh-style remotes.
func isSSHForm(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	at := strings.Index(s, "@")
	colon := strings.Index(s, ":")
	return at > 0 && colon > at
}

func sshHost(s string) string {
	// `git@host:path` → host
	at := strings.Index(s, "@")
	colon := strings.Index(s, ":")
	if at < 0 || colon <= at {
		return ""
	}
	return s[at+1 : colon]
}

// BuildCloneURL constructs an authenticated HTTPS clone URL based on the git provider.
func BuildCloneURL(provider, token, username, repoURL string) string {
	if !strings.HasPrefix(repoURL, "https://") || token == "" {
		return repoURL
	}
	switch provider {
	case "github":
		// GitHub App installation tokens authenticate as x-access-token.
		return strings.Replace(repoURL, "https://", "https://x-access-token:"+token+"@", 1)
	case "gitlab":
		return strings.Replace(repoURL, "https://", "https://oauth2:"+token+"@", 1)
	case "bitbucket":
		if username == "" {
			username = "x-token-auth"
		}
		return strings.Replace(repoURL, "https://", "https://"+username+":"+token+"@", 1)
	default: // gitea, generic PAT
		return strings.Replace(repoURL, "https://", "https://"+token+"@", 1)
	}
}

// Clone clones a git repository to the specified directory.
//
// Before invoking git, the repoURL is validated against an allowlist of
// schemes and a denylist of private/link-local network ranges. This is a
// best-effort SSRF mitigation: it cannot be perfect (DNS may flap, time-of
// check vs time-of-use), but it stops the obvious file://, http://localhost,
// and metadata-service exploits.
//
// If token is non-empty and repoURL is an HTTPS URL, the token is embedded
// as the username for authentication (e.g. GitHub PAT, GitLab token). The
// token is stripped from any error returned to the caller so it cannot leak
// into UI surfaces or logs at this layer.
func Clone(ctx context.Context, repoURL, destDir, branch, token string) (*CloneResult, error) {
	if err := validateRepoURL(repoURL, nil); err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("git clone: %s: %w", redactToken(string(output), token), redactError(err, token))
	}

	shaCmd := exec.CommandContext(ctx, "git", "-C", destDir, "rev-parse", "HEAD")
	shaOutput, err := shaCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}

	return &CloneResult{
		CommitSHA: strings.TrimSpace(string(shaOutput)),
	}, nil
}

// CloneCommit fetches a repository and checks out an exact commit SHA (rather
// than a branch tip). Used by Rebuild to re-build the currently-deployed commit
// instead of branch HEAD. It first tries a shallow fetch of the single commit
// (fast; supported by GitHub/GitLab and most modern servers), then falls back to
// a full clone + checkout for servers that reject fetching an arbitrary SHA.
// Token handling and SSRF validation match Clone.
func CloneCommit(ctx context.Context, repoURL, destDir, commitSHA, token string) (*CloneResult, error) {
	if err := validateRepoURL(repoURL, nil); err != nil {
		return nil, err
	}
	if !isHexSHA(commitSHA) {
		return nil, fmt.Errorf("invalid commit sha")
	}

	cloneURL := repoURL
	if token != "" && strings.HasPrefix(repoURL, "https://") {
		cloneURL = strings.Replace(repoURL, "https://", "https://"+token+"@", 1)
	}

	if err := shallowFetchCommit(ctx, cloneURL, destDir, commitSHA); err != nil {
		// Fallback: full clone + checkout works on any server, even those that
		// disallow fetching an arbitrary SHA. Reset destDir (the fetch attempt
		// git-init'd it) so `git clone` can recreate it.
		if rmErr := os.RemoveAll(destDir); rmErr != nil {
			return nil, fmt.Errorf("reset clone dir: %w", rmErr)
		}
		if cloneErr := fullCloneCheckout(ctx, cloneURL, destDir, commitSHA, token); cloneErr != nil {
			return nil, fmt.Errorf("clone commit %s (shallow fetch failed: %s): %w",
				commitSHA, redactToken(err.Error(), token), cloneErr)
		}
	}
	return resolveHead(ctx, destDir)
}

// shallowFetchCommit initialises destDir and shallow-fetches a single commit.
func shallowFetchCommit(ctx context.Context, cloneURL, destDir, commitSHA string) error {
	run := func(args ...string) error {
		out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	if err := run("init", "-q", destDir); err != nil {
		return err
	}
	if err := run("-C", destDir, "remote", "add", "origin", cloneURL); err != nil {
		return err
	}
	if err := run("-C", destDir, "fetch", "--depth", "1", "origin", commitSHA); err != nil {
		return err
	}
	return run("-C", destDir, "checkout", "-q", "FETCH_HEAD")
}

// fullCloneCheckout clones the full history then checks out the commit.
func fullCloneCheckout(ctx context.Context, cloneURL, destDir, commitSHA, token string) error {
	out, err := exec.CommandContext(ctx, "git", "clone", "--no-checkout", cloneURL, destDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %s: %w", redactToken(string(out), token), redactError(err, token))
	}
	out, err = exec.CommandContext(ctx, "git", "-C", destDir, "checkout", "-q", commitSHA).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout %s: %s: %w", commitSHA, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// resolveHead returns the checked-out commit SHA of a working tree.
func resolveHead(ctx context.Context, destDir string) (*CloneResult, error) {
	shaOutput, err := exec.CommandContext(ctx, "git", "-C", destDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	return &CloneResult{CommitSHA: strings.TrimSpace(string(shaOutput))}, nil
}

// isHexSHA reports whether s is a plausible git commit hash (7–64 hex chars).
func isHexSHA(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// redactToken removes occurrences of the auth token from a string before it
// flows back to the caller. Git error output occasionally echoes the
// authenticated URL (`https://<token>@host/...`); without redaction the
// token would show up in error fields surfaced to the UI.
func redactToken(s, token string) string {
	s = strings.TrimSpace(s)
	if token == "" || s == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}

// redactError wraps the underlying error with a token-stripped message.
func redactError(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), token, "[REDACTED]")
	return errors.New(msg)
}
