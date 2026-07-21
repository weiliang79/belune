// Package loglevel defines the canonical log-severity vocabulary used across
// every log surface (container logs, build logs, backup/restore logs) and a
// heuristic detector that assigns a level to an otherwise-unstructured log line.
//
// The detection order is deliberate and mirrored by the frontend
// (apps/web/src/lib/logs/level.ts) so a line renders with the same level on
// both sides:
//
//  1. an explicit "[LEVEL]" tag a producer stamped onto the line,
//  2. a structured JSON "level"/"severity" field,
//  3. a case-insensitive keyword scan of the message,
//  4. a fallback to Info. The originating stream is deliberately NOT treated as
//     a severity signal — see the note in Detect.
package loglevel

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Level is a canonical log severity. Values are lowercase so they can be stored
// verbatim in the container_logs.level column and compared without casing.
type Level string

const (
	Debug   Level = "debug"
	Info    Level = "info"
	Warning Level = "warning"
	Error   Level = "error"
)

// tagRe matches a leading "[LEVEL]" marker (with optional surrounding space)
// that a producer stamped onto a line via Tag. Anchored to the start so it only
// matches an intentional prefix, not the word appearing mid-message.
var tagRe = regexp.MustCompile(`^\s*\[(DEBUG|INFO|WARN|WARNING|ERROR)\]\s?`)

// keywordRe finds a severity keyword as a whole word anywhere in the message.
// Checked in Error > Warning > Debug > Info priority by inspecting the match.
// Failure words are part of the error set because severity now has to come from
// the message: the stream fallback no longer supplies it, so a line like
// "Failed to bind socket" must be recognised on its own content.
var (
	errRe   = regexp.MustCompile(`(?i)\b(fatal|error|err|panic|failed|failure|exception|traceback)\b`)
	warnRe  = regexp.MustCompile(`(?i)\b(warn|warning)\b`)
	debugRe = regexp.MustCompile(`(?i)\b(debug|trace)\b`)
	infoRe  = regexp.MustCompile(`(?i)\b(info|notice)\b`)
)

// Status glyphs many CLI/build tools (railpack, npm, pnpm, vite, …) prefix a
// line with instead of a word: a cross for failures, a warning sign for
// warnings. They're unambiguous, so they carry the same weight as the keywords
// above — without them a line like "✖ Failed to run mise command" (no "error"
// word) would fall through to Info.
var (
	errGlyphRe  = regexp.MustCompile(`[✖✗✘❌]`)
	warnGlyphRe = regexp.MustCompile(`⚠`)
)

// dbSeverityRe matches the colon-delimited severity prefix that databases and
// many app loggers emit, e.g. Postgres "LOG:  statement: ...", "WARNING: ...",
// "ERROR: ...". The trailing colon keeps it from matching casual mentions of
// these words. It runs before the keyword scan so an explicit "LOG:"/"DETAIL:"
// prefix wins over an incidental failure word later in the same line.
var dbSeverityRe = regexp.MustCompile(`(?i)\b(log|notice|info|detail|hint|context|statement|warning|error|fatal|panic)\s*:`)

// Normalize maps an arbitrary level string (any casing, common aliases) to a
// canonical Level. Unknown values return Info.
func Normalize(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "trace", "dbg":
		return Debug
	case "info", "information", "notice", "log":
		return Info
	case "warn", "warning":
		return Warning
	case "error", "err", "fatal", "critical", "crit", "panic":
		return Error
	default:
		return Info
	}
}

// Detect assigns a level to a single log line. stream is the originating stream
// ("stdout"/"stderr"/"") used only as a last-resort fallback.
func Detect(message, stream string) Level {
	// 1. Explicit tag stamped by a producer.
	if m := tagRe.FindStringSubmatch(message); m != nil {
		return Normalize(m[1])
	}

	// 2. Structured JSON with a level/severity field.
	if lvl, ok := jsonLevel(message); ok {
		return lvl
	}

	// 3. Colon-delimited severity prefix (Postgres/MySQL and many app loggers).
	if m := dbSeverityRe.FindStringSubmatch(message); m != nil {
		// LOG/DETAIL/HINT/CONTEXT/STATEMENT normalize to Info.
		switch strings.ToLower(m[1]) {
		case "log", "detail", "hint", "context", "statement":
			return Info
		default:
			return Normalize(m[1])
		}
	}

	// 4. Explicit status glyphs. These are producer intent — as deliberate as a
	// "[LEVEL]" tag — so they outrank the inferred keywords below. Without this
	// ordering a line the tool itself marked as a warning, e.g.
	// "⚠ Failed to get package versions", would be forced to Error by the word
	// "Failed".
	switch {
	case errGlyphRe.MatchString(message):
		return Error
	case warnGlyphRe.MatchString(message):
		return Warning
	}

	// 5. Inferred keyword scan, highest severity wins.
	switch {
	case errRe.MatchString(message):
		return Error
	case warnRe.MatchString(message):
		return Warning
	case debugRe.MatchString(message):
		return Debug
	case infoRe.MatchString(message):
		return Info
	}

	// 6. Fallback.
	//
	// stderr is deliberately NOT treated as Error. It is a stream, not a
	// severity: Go's standard log package, Python's logging, and many CLIs send
	// ordinary informational output there, so "stderr => Error" mislabelled
	// perfectly healthy lines — traefik/whoami's "Starting up on port 80" being
	// the canonical example. Genuine failures are identified from the message
	// itself in the steps above.
	return Info
}

// jsonLevel parses a line that looks like a JSON object and extracts a
// level/severity field if present.
func jsonLevel(message string) (Level, bool) {
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return "", false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return "", false
	}
	for _, key := range []string{"level", "severity", "lvl", "levelname"} {
		if raw, ok := obj[key]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil && s != "" {
				return Normalize(s), true
			}
		}
	}
	return "", false
}
