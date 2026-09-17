package sse

import (
	"fmt"
	"net/http"
)

// Writer wraps an http.ResponseWriter for Server-Sent Events.
//
// ⚠️ These streams are NOT deprecated, despite a v0.0.4-alpha commit that
// stamped every response with `X-Deprecated: use WebSocket /api/ws`. That was
// written when the dashboard was the only client, and PATs made script authors
// a first-class audience in v0.1.6 — for them SSE is one curl with no client
// library, while the hub needs a WebSocket client, a subscribe frame, and a
// message loop.
//
// The two also answer different questions, so the hub is not a replacement:
// StreamLogs reads the CONTAINER's own log buffer (Docker LogsOptions with Tail
// unset, i.e. "all"), so it returns full history and then optionally follows.
// The hub's container-logs channel is fed by the log collector over Redis and
// carries only what arrives after you subscribe. Dropping SSE would remove the
// only way to read a container's log history over the API — logs/history is the
// collector's database view, bounded by app_log_retention_days, not the same
// thing.
//
// The header also pointed callers of /api/notifications/stream at a channel the
// hub does not have.
type Writer struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewWriter creates a new SSE writer and sets appropriate headers.
func NewWriter(w http.ResponseWriter) (*Writer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &Writer{w: w, flusher: flusher}, nil
}

// SendEvent writes a named SSE event.
func (s *Writer) SendEvent(event, data string) error {
	_, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendComment writes an SSE comment, which keeps the connection alive and
// forces proxies to flush headers without triggering a client-side event.
func (s *Writer) SendComment(text string) error {
	_, err := fmt.Fprintf(s.w, ": %s\n\n", text)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendData writes an SSE data-only message.
func (s *Writer) SendData(data string) error {
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
