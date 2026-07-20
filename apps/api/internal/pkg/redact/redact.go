package redact

import (
	"regexp"
	"strings"
)

// secretPathPrefixes are route prefixes whose NEXT path segment is a credential
// rather than an identifier. A token in the URL is the ergonomic that makes a
// deploy hook a one-line curl, but it also means the secret rides in the
// request line — which is exactly what access logs record. Anything added here
// must be a prefix under which the following segment is secret.
var secretPathPrefixes = []string{
	"/api/webhooks/deploy/",
}

// Path sanitises a URL path for logging, replacing a credential-bearing
// segment with [REDACTED]. It only touches the one segment after a known
// prefix, so the route itself stays greppable in logs.
func Path(path string) string {
	for _, prefix := range secretPathPrefixes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		rest := path[len(prefix):]
		if rest == "" {
			return path
		}
		// Preserve anything after the secret segment (a trailing slash or a
		// deeper path) so a mistyped URL still shows its shape in the log.
		if idx := strings.IndexAny(rest, "/?"); idx >= 0 {
			return prefix + "[REDACTED]" + rest[idx:]
		}
		return prefix + "[REDACTED]"
	}
	return path
}

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
