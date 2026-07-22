package tracing

import (
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
)

// defaultErrorWindow is how long an identical, repeating OTel error is
// suppressed before it is reported again with a count.
const defaultErrorWindow = time.Minute

// SetErrorHandler routes OpenTelemetry's internal errors through slog at Error
// level, throttling identical repeats.
//
// Without it those errors reach the standard log package, which slog.SetDefault
// bridges at Info — so an exporter that cannot reach its collector reports
//
//	INFO [stdlog] traces export: Post "http://jaeger:4318/v1/traces": ... no such host
//
// An export failure is not information, and at Info it is invisible to every
// error filter in the product, including the log viewer's own.
//
// The throttle exists because the failure repeats on the exporter's schedule —
// roughly every 10s, about 8,600 times a day against a collector that is simply
// not there. Promoting that to Error unthrottled would bury every real error
// behind it and make error-level filtering useless, which is worse than the
// wrong level. The first occurrence is always reported immediately; repeats are
// summarised once per window.
func SetErrorHandler() {
	otel.SetErrorHandler(newThrottledHandler(defaultErrorWindow, time.Now))
}

type throttledHandler struct {
	mu         sync.Mutex
	window     time.Duration
	now        func() time.Time
	lastMsg    string
	lastAt     time.Time
	suppressed int
}

func newThrottledHandler(window time.Duration, now func() time.Time) *throttledHandler {
	return &throttledHandler{window: window, now: now}
}

func (h *throttledHandler) Handle(err error) {
	if err == nil {
		return
	}
	msg := err.Error()

	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now()
	// A different error is always news, even inside the window: suppressing it
	// would hide a new failure behind an old one.
	if msg != h.lastMsg {
		h.flushLocked()
		h.lastMsg, h.lastAt, h.suppressed = msg, now, 0
		slog.Error("otel error", "error", msg)
		return
	}

	if now.Sub(h.lastAt) < h.window {
		h.suppressed++
		return
	}

	repeats := h.suppressed
	h.lastAt, h.suppressed = now, 0
	if repeats > 0 {
		slog.Error("otel error", "error", msg, "repeated", repeats)
		return
	}
	slog.Error("otel error", "error", msg)
}

// flushLocked reports anything held back for the previous message, so a
// suppressed run is never silently dropped when the error changes.
func (h *throttledHandler) flushLocked() {
	if h.suppressed > 0 && h.lastMsg != "" {
		slog.Error("otel error", "error", h.lastMsg, "repeated", h.suppressed)
	}
	h.suppressed = 0
}
