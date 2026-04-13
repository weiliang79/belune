package redact

import "regexp"

var patterns = []*regexp.Regexp{
	// https://token@host or https://user:pass@host (git credentials in URLs)
	regexp.MustCompile(`https?://[^@\s]+@`),
	// Generic key=value credential patterns (token, password, secret, key, credential)
	regexp.MustCompile(`(?i)(token|password|secret|key|credential)[\s=:]+\S+`),
	// Bearer / JWT tokens in Authorization headers or log lines
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`),
	// API keys in query strings or headers (api_key=, apikey=, x-api-key:)
	regexp.MustCompile(`(?i)(api[_\-]?key|x[_\-]?api[_\-]?key)[=:\s]+[^\s&"]+`),
	// Webhook signatures (X-Hub-Signature, X-Signature, X-Webhook-Secret)
	regexp.MustCompile(`(?i)(x[_\-]?(hub[_\-]?signature|signature|webhook[_\-]?secret))[=:\s]+[^\s"]+`),
}

// Error sanitises an error message by replacing likely credentials with [REDACTED].
func Error(msg string) string {
	for _, p := range patterns {
		msg = p.ReplaceAllString(msg, "[REDACTED]")
	}
	return msg
}
