package loglevel

import "testing"

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		message string
		stream  string
		want    Level
	}{
		// Explicit tag wins over everything else.
		{"tag error", "[ERROR] something happened", "stdout", Error},
		{"tag warn", "[WARN] heads up", "stdout", Warning},
		{"tag warning alias", "[WARNING] heads up", "stdout", Warning},
		{"tag info", "[INFO] all good", "stderr", Info},
		{"tag debug", "[DEBUG] verbose", "stdout", Debug},
		{"tag beats keyword", "[INFO] this error is not really an error", "stdout", Info},

		// JSON structured level.
		{"json level", `{"level":"debug","msg":"x"}`, "stdout", Debug},
		{"json severity", `{"severity":"WARNING","message":"x"}`, "stdout", Warning},
		{"json levelname", `{"levelname":"ERROR"}`, "stdout", Error},
		{"json not object", `[1,2,3]`, "stdout", Info},

		// Colon-delimited DB / logger severity (databases log to stderr).
		{"pg log on stderr", "2024-01-01 00:00:00 UTC [1] LOG:  database system is ready", "stderr", Info},
		{"pg error on stderr", "2024-01-01 00:00:00 UTC [1] ERROR:  relation does not exist", "stderr", Error},
		{"pg fatal on stderr", "2024-01-01 00:00:00 UTC [1] FATAL:  password authentication failed", "stderr", Error},
		{"pg warning on stderr", `2024-01-01 00:00:00 UTC [1] WARNING:  there is already a transaction`, "stderr", Warning},
		{"pg detail is info", "DETAIL:  Key (id)=(1) already exists.", "stderr", Info},

		// Keyword scan.
		{"keyword error", "connection ERROR: refused", "stdout", Error},
		{"keyword fatal", "fatal: could not read", "stdout", Error},
		{"keyword panic", "panic: nil pointer", "stdout", Error},
		{"keyword warn", "WARN missing lockfile", "stdout", Warning},
		{"keyword debug", "debug: entering loop", "stdout", Debug},
		{"keyword info", "INFO server listening", "stdout", Info},
		{"keyword priority error over warn", "error while handling warn state", "stdout", Error},

		// Status glyphs used by build tools (railpack et al.) instead of words.
		{"glyph cross is error", "✖ Failed to run mise command 'mise latest node@22': exit status 1", "", Error},
		{"glyph warn sign is warning", "⚠ Failed to get package versions from mise", "", Warning},
		{"glyph heavy cross is error", "✘ build step failed", "stdout", Error},
		{"glyph error beats warn glyph", "⚠ ✖ both present", "", Error},
		{"substring not matched", "erroneous is not error-word", "stdout", Error}, // contains "error" whole word? no -> "erroneous" no, but "error-word" has error boundary
		{"no keyword plain", "just a normal line", "stdout", Info},

		// Stream fallback.
		{"stderr fallback", "some opaque output", "stderr", Error},
		{"stdout fallback", "some opaque output", "stdout", Info},
		{"empty stream fallback", "some opaque output", "", Info},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.message, tt.stream); got != tt.want {
				t.Errorf("Detect(%q, %q) = %q, want %q", tt.message, tt.stream, got, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]Level{
		"DEBUG": Debug, "trace": Debug,
		"Info": Info, "notice": Info, "log": Info,
		"WARN": Warning, "warning": Warning,
		"error": Error, "FATAL": Error, "critical": Error, "panic": Error,
		"":        Info,
		"unknown": Info,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
