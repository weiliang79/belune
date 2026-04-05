package redact

import "regexp"

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^@\s]+@`),                               // https://token@host or https://user:pass@host
	regexp.MustCompile(`(?i)(token|password|secret|key|credential)[\s=:]+\S+`), // key=value patterns
}

// Error sanitises an error message by replacing likely credentials with [REDACTED].
func Error(msg string) string {
	for _, p := range patterns {
		msg = p.ReplaceAllString(msg, "[REDACTED]")
	}
	return msg
}
