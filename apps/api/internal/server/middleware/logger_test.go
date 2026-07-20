package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The deploy-hook token is a live credential carried in the URL path, and this
// middleware writes the path to stdout — wherever that is shipped. Assert on
// the emitted log record rather than on redact.Path, so the wiring itself is
// covered: an unredacted r.URL.Path here would leak working tokens.
func TestLogger_RedactsDeployHookToken(t *testing.T) {
	const token = "B8LoxFsJaaXjhJxQP0gA9SUmkstisGZ_5KJWK4ycKxk"

	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/deploy/"+token, nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	if strings.Contains(logged, token) {
		t.Fatalf("deploy hook token leaked into the request log: %s", logged)
	}
	if !strings.Contains(logged, "/api/webhooks/deploy/[REDACTED]") {
		t.Fatalf("expected redacted path in log, got: %s", logged)
	}
}

// Ordinary paths must survive untouched, or the logs stop being useful.
func TestLogger_LeavesNormalPathsIntact(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/push", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), "/api/webhooks/push") {
		t.Fatalf("expected unmodified path in log, got: %s", buf.String())
	}
}
