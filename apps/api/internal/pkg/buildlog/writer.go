package buildlog

import (
	"bytes"
	"context"
	"time"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/joblog"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/loglevel"
)

// LineWriter implements io.Writer over the build output stream. It buffers
// partial lines until a newline, then for each complete line: infers a level,
// publishes the line as an NDJSON entry to the Publisher (for live viewers),
// and accumulates the same NDJSON for the stored build log. Storing the level
// and timestamp as fields (rather than raw text) lets the log viewer localize
// the time and filter by level like every other log surface.
type LineWriter struct {
	pub *Publisher
	ctx context.Context
	buf bytes.Buffer
	log joblog.Builder
}

func NewLineWriter(pub *Publisher, ctx context.Context) *LineWriter {
	return &LineWriter{pub: pub, ctx: ctx}
}

func (w *LineWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil {
			// No complete line yet — put the partial data back
			w.buf.Write(line)
			break
		}
		w.emit(string(bytes.TrimRight(line, "\n\r")))
	}
	return len(p), nil
}

// Flush publishes any remaining buffered content.
func (w *LineWriter) Flush() {
	if w.buf.Len() > 0 {
		remaining := w.buf.String()
		w.buf.Reset()
		w.emit(remaining)
	}
}

// NDJSON returns the accumulated build log as newline-delimited JSON entries.
func (w *LineWriter) NDJSON() string { return w.log.String() }

func (w *LineWriter) emit(line string) {
	if line == "" {
		return
	}
	e := joblog.Entry{Ts: time.Now().UTC(), Level: loglevel.Detect(line, ""), Msg: line}
	w.log.AddEntry(e)
	// Don't fail the build if a Redis publish fails.
	_ = w.pub.Publish(w.ctx, e.MarshalLine())
}
