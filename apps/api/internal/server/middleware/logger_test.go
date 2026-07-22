package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// The message is the column a reader scans. It used to be the literal word
// "request" on every line, with the content pushed into attributes — fourteen
// identical rows in the log viewer.
func TestLogger_MessageCarriesTheRequest(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/features", nil))

	out := buf.String()
	if !strings.Contains(out, `msg="GET /api/features 200`) {
		t.Fatalf("expected method, path and status in the message, got: %s", out)
	}
	if strings.Contains(out, `msg=request`) {
		t.Fatalf("message is still the placeholder: %s", out)
	}
}

// status and duration stay queryable for LOG_FORMAT=json: Belune's own API
// traffic is not in request_logs, which the Caddy tailer fills per application.
func TestLogger_KeepsStatusAndDurationQueryable(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	out := buf.String()
	for _, want := range []string{"status=200", "duration_ms=", "request_id="} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s as an attribute, got: %s", want, out)
		}
	}
}

// A 500 logged at Info sits below every filter meant to surface it.
func TestLogger_ServerErrorsLogAtErrorLevel(t *testing.T) {
	cases := []struct {
		status    int
		wantLevel string
	}{
		{http.StatusOK, "level=INFO"},
		{http.StatusNotFound, "level=INFO"},     // client errors are ordinary traffic
		{http.StatusUnauthorized, "level=INFO"}, // an unauthenticated probe is not our problem
		{http.StatusInternalServerError, "level=ERROR"},
		{http.StatusBadGateway, "level=ERROR"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		original := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

		handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
		slog.SetDefault(original)

		if !strings.Contains(buf.String(), tc.wantLevel) {
			t.Fatalf("status %d: expected %s, got: %s", tc.status, tc.wantLevel, buf.String())
		}
	}
}

// Duration.String() prints full nanosecond precision ("1.000054247s"), which is
// a dozen characters of noise on a line meant to be skimmed.
func TestFormatLatency(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{1500 * time.Millisecond, "1.5s"},
		{time.Second + 54*time.Microsecond, "1.0s"},
		{926*time.Millisecond + 143*time.Microsecond, "926ms"},
		{1942 * time.Microsecond, "2ms"},
		{556 * time.Microsecond, "556µs"},
	}
	for _, tc := range cases {
		if got := formatLatency(tc.in); got != tc.want {
			t.Errorf("formatLatency(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
