package git

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCloneURL_GitHub(t *testing.T) {
	url := BuildCloneURL("github", "ghp_token123", "", "https://github.com/user/repo.git")
	assert.Equal(t, "https://ghp_token123@github.com/user/repo.git", url)
}

func TestBuildCloneURL_GitLab(t *testing.T) {
	url := BuildCloneURL("gitlab", "glpat-token123", "", "https://gitlab.com/user/repo.git")
	assert.Equal(t, "https://oauth2:glpat-token123@gitlab.com/user/repo.git", url)
}

func TestBuildCloneURL_Bitbucket(t *testing.T) {
	url := BuildCloneURL("bitbucket", "app_password", "myuser", "https://bitbucket.org/user/repo.git")
	assert.Equal(t, "https://myuser:app_password@bitbucket.org/user/repo.git", url)
}

func TestBuildCloneURL_BitbucketDefaultUsername(t *testing.T) {
	url := BuildCloneURL("bitbucket", "app_password", "", "https://bitbucket.org/user/repo.git")
	assert.Equal(t, "https://x-token-auth:app_password@bitbucket.org/user/repo.git", url)
}

func TestBuildCloneURL_Generic(t *testing.T) {
	url := BuildCloneURL("generic", "mytoken", "", "https://gitea.example.com/user/repo.git")
	assert.Equal(t, "https://mytoken@gitea.example.com/user/repo.git", url)
}

func TestBuildCloneURL_NoToken(t *testing.T) {
	url := BuildCloneURL("github", "", "", "https://github.com/user/repo.git")
	assert.Equal(t, "https://github.com/user/repo.git", url)
}

func TestBuildCloneURL_SSHPassthrough(t *testing.T) {
	url := BuildCloneURL("github", "token", "", "git@github.com:user/repo.git")
	assert.Equal(t, "git@github.com:user/repo.git", url)
}

// fakeResolver returns canned IPs by hostname. Tests use this to avoid live
// DNS lookups and to assert the IP-based block list separately from the
// hostname checks.
func fakeResolver(m map[string][]string) func(string) ([]net.IP, error) {
	return func(host string) ([]net.IP, error) {
		ips, ok := m[host]
		if !ok {
			return nil, errors.New("no entry")
		}
		var out []net.IP
		for _, s := range ips {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
}

func TestValidateRepoURL_AcceptsPublicHTTPS(t *testing.T) {
	resolver := fakeResolver(map[string][]string{
		"github.com": {"140.82.121.4"},
	})
	err := validateRepoURL("https://github.com/foo/bar.git", resolver)
	assert.NoError(t, err)
}

func TestValidateRepoURL_AcceptsSSHForm(t *testing.T) {
	resolver := fakeResolver(map[string][]string{
		"github.com": {"140.82.121.4"},
	})
	err := validateRepoURL("git@github.com:foo/bar.git", resolver)
	assert.NoError(t, err)
}

func TestValidateRepoURL_RejectsFileScheme(t *testing.T) {
	err := validateRepoURL("file:///etc/passwd", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsLocalhostByName(t *testing.T) {
	err := validateRepoURL("https://localhost/foo.git", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsIPLiteralLoopback(t *testing.T) {
	err := validateRepoURL("https://127.0.0.1/foo.git", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsIPv6Loopback(t *testing.T) {
	err := validateRepoURL("https://[::1]/foo.git", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsRFC1918(t *testing.T) {
	cases := []string{
		"https://10.0.0.1/foo.git",
		"https://172.16.5.5/foo.git",
		"https://192.168.1.1/foo.git",
	}
	for _, c := range cases {
		err := validateRepoURL(c, nil)
		assert.ErrorIs(t, err, ErrUnsafeRepoURL, c)
	}
}

func TestValidateRepoURL_RejectsLinkLocalMetadata(t *testing.T) {
	// AWS / GCP / DigitalOcean cloud metadata service.
	err := validateRepoURL("https://169.254.169.254/latest/meta-data/", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsDNSResolvingToPrivate(t *testing.T) {
	resolver := fakeResolver(map[string][]string{
		"evil.example.com": {"10.0.0.1"},
	})
	err := validateRepoURL("https://evil.example.com/foo.git", resolver)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsResolverFailure(t *testing.T) {
	resolver := fakeResolver(map[string][]string{}) // empty map, all lookups fail
	err := validateRepoURL("https://nope.example.com/foo.git", resolver)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestValidateRepoURL_RejectsGopher(t *testing.T) {
	err := validateRepoURL("gopher://example.com/foo", nil)
	assert.ErrorIs(t, err, ErrUnsafeRepoURL)
}

func TestRedactToken(t *testing.T) {
	out := redactToken("error: failed to push to https://ghp_secret123@github.com/foo/bar.git", "ghp_secret123")
	assert.NotContains(t, out, "ghp_secret123")
	assert.Contains(t, out, "[REDACTED]")
}
