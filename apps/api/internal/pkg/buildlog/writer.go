package buildlog

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/weiling79/belune/internal/pkg/joblog"
	"github.com/weiling79/belune/internal/pkg/loglevel"
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
	mu  sync.Mutex // guards log (read by NDJSON from the periodic flusher goroutine)
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
// Safe to call concurrently with Write (e.g. from a periodic flusher).
func (w *LineWriter) NDJSON() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.log.String()
}

func (w *LineWriter) emit(line string) {
	if line == "" {
		return
	}
	e := joblog.Entry{Ts: time.Now().UTC(), Level: loglevel.Detect(line, ""), Msg: line}
	w.mu.Lock()
	w.log.AddEntry(e)
	w.mu.Unlock()
	// Don't fail the build if a Redis publish fails.
	_ = w.pub.Publish(w.ctx, e.MarshalLine())
}
