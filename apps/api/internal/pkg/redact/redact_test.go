package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_RedactsHTTPSTokenURL(t *testing.T) {
	msg := "git clone: https://ghp_abc123@github.com/user/repo.git: exit status 128"
	result := Error(msg)
	assert.NotContains(t, result, "ghp_abc123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestError_RedactsHTTPTokenURL(t *testing.T) {
	msg := "failed: http://token123@gitlab.com/repo"
	result := Error(msg)
	assert.NotContains(t, result, "token123")
	assert.Contains(t, result, "[REDACTED]")
}

func TestError_RedactsUserPassURL(t *testing.T) {
	msg := "clone failed: https://user:pass@bitbucket.org/repo"
	result := Error(msg)
	assert.NotContains(t, result, "user:pass")
	assert.Contains(t, result, "[REDACTED]")
}

func TestError_RedactsKeyValuePatterns(t *testing.T) {
	tests := []struct {
		input    string
		redacted string
	}{
		{"token=abc123def", "[REDACTED]"},
		{"password: mysecret", "[REDACTED]"},
		{"SECRET=very-secret-value", "[REDACTED]"},
	}
	for _, tc := range tests {
		result := Error(tc.input)
		assert.Contains(t, result, "[REDACTED]", "should redact: %s", tc.input)
	}
}

func TestError_RedactsBearerTokens(t *testing.T) {
	tests := []string{
		"Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
		"bearer abc123token",
		"BEARER eyABC==",
	}
	for _, input := range tests {
		result := Error(input)
		assert.Contains(t, result, "[REDACTED]", "should redact bearer token: %s", input)
	}
}

func TestError_RedactsAPIKeys(t *testing.T) {
	tests := []string{
		"request failed: api_key=sk-abc123",
		"sending with apikey: supersecret",
		"header x-api-key: myapikey123",
	}
	for _, input := range tests {
		result := Error(input)
		assert.Contains(t, result, "[REDACTED]", "should redact API key: %s", input)
	}
}

func TestError_RedactsWebhookSignatures(t *testing.T) {
	tests := []string{
		"x-hub-signature: sha256=abc123def456",
		"x-signature=deadbeef",
		"X-Webhook-Secret: mysecret",
	}
	for _, input := range tests {
		result := Error(input)
		assert.Contains(t, result, "[REDACTED]", "should redact webhook signature: %s", input)
	}
}

func TestError_PreservesCleanMessages(t *testing.T) {
	msg := "git clone: repository not found: exit status 128"
	result := Error(msg)
	assert.Equal(t, msg, result)
}
