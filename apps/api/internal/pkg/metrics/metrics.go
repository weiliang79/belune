// Package metrics exposes Prometheus instrumentation for the PaaS API.
//
// A single process-wide registry holds every metric. Call Handler() to obtain
// the /metrics HTTP handler, HTTPMiddleware() to record per-request counters,
// and the RecordX helpers to report subsystem events from call sites.
//
// All metrics are registered once at package init. Subsystems that are unsure
// whether metrics are enabled can call the RecordX helpers unconditionally —
// the default prometheus.Registerer guarantees cheap no-op observations when
// the /metrics endpoint is never scraped.
package metrics

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the package-level registry used for every PaaS metric. Exposed
// for tests and for callers who want to register extra collectors.
var Registry = prometheus.NewRegistry()

var (
	factory = promauto.With(Registry)

	httpRequestsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "belune_http_requests_total",
		Help: "HTTP requests served by the Belune API, labelled by chi route pattern and status class.",
	}, []string{"method", "pattern", "status"})

	httpRequestDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "belune_http_request_duration_seconds",
		Help:    "Latency of HTTP requests served by the Belune API.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "pattern"})

	asynqQueueSize = factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "belune_asynq_queue_size",
		Help: "Asynq queue depth by queue name and task state.",
	}, []string{"queue", "state"})

	deployDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "belune_deploy_stage_duration_seconds",
		Help:    "Deploy pipeline stage durations (build, push, run, route, verify).",
		Buckets: []float64{0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
	}, []string{"stage", "status"})

	dockerOps = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "belune_docker_runtime_operations_total",
		Help: "Docker runtime operations by op name and result.",
	}, []string{"op", "result"})

	caddyAdminCalls = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "belune_caddy_admin_calls_total",
		Help: "Caddy admin API calls by op name and result.",
	}, []string{"op", "result"})
)

func init() {
	// Go runtime + process (RSS, CPU, fds, ...) collectors. Registered on the
	// package registry so promhttp.HandlerFor emits them alongside our custom
	// metrics.
	Registry.MustRegister(collectors.NewGoCollector())
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// Handler returns the /metrics HTTP handler backed by the package Registry.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry:          Registry,
		EnableOpenMetrics: true,
	})
}

// HTTPMiddleware records request counts and durations keyed by the chi route
// pattern (not the raw URL path) to keep cardinality bounded.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		pattern := chi.RouteContext(r.Context()).RoutePattern()
		if pattern == "" {
			pattern = "unknown"
		}
		httpRequestsTotal.WithLabelValues(r.Method, pattern, strconv.Itoa(ww.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, pattern).Observe(time.Since(start).Seconds())
	})
}

// statusRecorder captures the final response status code for metric labels
// without otherwise interfering with the response lifecycle.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Hijack delegates to the underlying ResponseWriter so WebSocket upgrades work
// through this middleware. Without it, the Hijacker interface assertion fails
// at this wrapper and upgrades surface as "Invalid frame header" in the client.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Flush delegates to the underlying ResponseWriter so SSE / streaming handlers
// can flush through this middleware.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RecordDeployStage observes stage duration + status. status is typically
// "ok" or "error"; callers should normalise error-ness before passing.
func RecordDeployStage(stage, status string, d time.Duration) {
	deployDuration.WithLabelValues(stage, status).Observe(d.Seconds())
}

// RecordDockerOp increments the counter for a Docker runtime op. Pass err from
// the underlying call — nil → "ok", non-nil → "error".
func RecordDockerOp(op string, err error) {
	dockerOps.WithLabelValues(op, resultLabel(err)).Inc()
}

// RecordCaddyCall increments the counter for a Caddy admin API op.
func RecordCaddyCall(op string, err error) {
	caddyAdminCalls.WithLabelValues(op, resultLabel(err)).Inc()
}

// SetAsynqQueueSize reports the current queue depth for one queue/state pair.
// Called from a periodic poller; overwrites any previous value.
func SetAsynqQueueSize(queue, state string, size int) {
	asynqQueueSize.WithLabelValues(queue, state).Set(float64(size))
}

// resultLabel maps an error to the two-valued label used by op counters.
func resultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	// Context cancellations are part of normal shutdown and shouldn't spike
	// the error rate dashboard.
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "error"
}
