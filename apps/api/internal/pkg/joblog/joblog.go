// Package joblog builds NDJSON log output for operation/run logs (deployments,
// database backups and restores). Each log line is a JSON object
//
//	{"ts":"2006-01-02T15:04:05.000Z","level":"info","msg":"…"}
//
// stored one-per-line in the run's TEXT column. Storing the timestamp and level
// as real fields (instead of embedding them in the message text) lets the log
// viewer localize the time and align/filter by level exactly like container
// logs. The format mirrors the frontend parser in
// apps/web/src/components/logs/parse.ts.
package joblog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/loglevel"
)

// Entry is a single log line.
type Entry struct {
	Ts    time.Time      `json:"ts"`
	Level loglevel.Level `json:"level"`
	Msg   string         `json:"msg"`
}

// MarshalLine renders one entry as a single NDJSON line (no trailing newline).
func (e Entry) MarshalLine() string {
	b, err := json.Marshal(e)
	if err != nil {
		// Fall back to a minimal valid object so a bad line never corrupts the
		// stream or loses the message.
		b, _ = json.Marshal(Entry{Ts: e.Ts, Level: loglevel.Info, Msg: e.Msg})
	}
	return string(b)
}

// Builder accumulates entries and renders them as NDJSON. It is not safe for
// concurrent use; callers serialize writes (the backup runLog and build
// LineWriter both do).
type Builder struct {
	sb strings.Builder
}

// Add appends an entry with an explicit level, stamped at the current time.
func (b *Builder) Add(level loglevel.Level, msg string) {
	b.add(Entry{Ts: time.Now().UTC(), Level: level, Msg: msg})
}

// AddDetected appends an entry whose level is inferred from the message and the
// originating stream (used for verbatim tool/build output).
func (b *Builder) AddDetected(stream, msg string) {
	b.add(Entry{Ts: time.Now().UTC(), Level: loglevel.Detect(msg, stream), Msg: msg})
}

// AddRaw splits verbatim multi-line output into one detected-level entry per
// non-empty line. Handy for captured command stderr/stdout.
func (b *Builder) AddRaw(stream, text string) {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			b.AddDetected(stream, line)
		}
	}
}

// AddEntry appends a pre-built entry.
func (b *Builder) AddEntry(e Entry) { b.add(e) }

func (b *Builder) add(e Entry) {
	b.sb.WriteString(e.MarshalLine())
	b.sb.WriteByte('\n')
}

// String returns the accumulated NDJSON.
func (b *Builder) String() string { return b.sb.String() }

// Len reports how many bytes have been accumulated.
func (b *Builder) Len() int { return b.sb.Len() }
