package git

import (
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
