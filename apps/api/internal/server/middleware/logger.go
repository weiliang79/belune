package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/weiliang79/belune/internal/pkg/redact"
)

// Logger logs each HTTP request with method, path, status, duration, and request ID.
// It relies on Chi's RequestID middleware having run first.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		slog.Info("request",
			"method", r.Method,
			// Redacted: some routes carry a credential in the path itself, and
			// this line is shipped wherever stdout goes.
			"path", redact.Path(r.URL.Path),
			"status", ww.statusCode,
			"duration", time.Since(start).String(),
			"request_id", chiMiddleware.GetReqID(r.Context()),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets the WebSocket handler take over the connection. Without this
// method the wrapped ResponseWriter doesn't satisfy http.Hijacker and the
// WS upgrade fails.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}
