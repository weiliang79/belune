package buildlog

import (
	"bytes"
	"context"
)

// LineWriter implements io.Writer and publishes complete lines to a Publisher.
// It buffers partial lines until a newline is received.
type LineWriter struct {
	pub *Publisher
	ctx context.Context
	buf bytes.Buffer
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
		// Trim the trailing newline and publish
		trimmed := bytes.TrimRight(line, "\n\r")
		if len(trimmed) > 0 {
			if pubErr := w.pub.Publish(w.ctx, string(trimmed)); pubErr != nil {
				// Don't fail the build if Redis publish fails
				continue
			}
		}
	}
	return len(p), nil
}

// Flush publishes any remaining buffered content.
func (w *LineWriter) Flush() {
	if w.buf.Len() > 0 {
		remaining := w.buf.String()
		w.buf.Reset()
		if len(remaining) > 0 {
			w.pub.Publish(w.ctx, remaining)
		}
	}
}
