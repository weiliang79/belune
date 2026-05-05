package proxy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// Reconciler periodically compares DB-declared routes against Caddy's live
// route table and reconciles any drift by adding missing routes and removing
// stale ones. It replaces the one-shot recoverProxyRoutes startup call.
type Reconciler struct {
	queries  *generated.Queries
	proxy    ProxyManager
	interval time.Duration

	mu     sync.RWMutex
	status ReconcilerStatus
}

// ReconcilerStatus is a snapshot of the reconciler's recent activity. It is
// exposed over the admin API so operators can tell at a glance whether the
// background loop is still healthy and whether Caddy has been drifting.
type ReconcilerStatus struct {
	IntervalSeconds int       `json:"interval_seconds"`
	LastRunAt       time.Time `json:"last_run_at"`
	LastDurationMs  int64     `json:"last_duration_ms"`
	LastAdded       int       `json:"last_added"`
	LastRemoved     int       `json:"last_removed"`
	LastError       string    `json:"last_error,omitempty"`
	RunCount        int64     `json:"run_count"`
	// TotalDrift counts every route that had to be added or removed across
	// all runs. Stable value over time means Caddy is in sync with the DB.
	TotalDrift int64 `json:"total_drift"`
}

// NewReconciler creates a Reconciler with the given tick interval.
func NewReconciler(queries *generated.Queries, proxy ProxyManager, interval time.Duration) *Reconciler {
	return &Reconciler{
		queries:  queries,
		proxy:    proxy,
		interval: interval,
		status:   ReconcilerStatus{IntervalSeconds: int(interval / time.Second)},
	}
}

// Status returns a snapshot of the reconciler's latest run state.
func (r *Reconciler) Status() ReconcilerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// Run starts the reconcile loop. It reconciles immediately on entry, then on
// every tick. Returns when ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	r.reconcile(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile performs one reconciliation pass. Errors are logged and retried on
// the next tick — reconciliation is best-effort and must not block startup.
// The outcome (added/removed counts, first error) is recorded on the
// Reconciler so it can be surfaced via Status().
func (r *Reconciler) reconcile(ctx context.Context) {
	start := time.Now()
	var added, removed int
	var firstErr error
	recordErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	defer func() {
		r.mu.Lock()
		r.status.LastRunAt = start
		r.status.LastDurationMs = time.Since(start).Milliseconds()
		r.status.LastAdded = added
		r.status.LastRemoved = removed
		r.status.RunCount++
		r.status.TotalDrift += int64(added + removed)
		if firstErr != nil {
			r.status.LastError = firstErr.Error()
		} else {
			r.status.LastError = ""
		}
		r.mu.Unlock()
	}()

	expected, err := r.buildExpected(ctx)
	if err != nil {
		slog.Warn("proxy reconciler: failed to build expected routes", "error", err)
		recordErr(err)
		return
	}

	current, err := r.proxy.ListRoutes(ctx)
	if err != nil {
		slog.Warn("proxy reconciler: failed to list current routes", "error", err)
		recordErr(err)
		return
	}

	// Index current routes by hostname.
	currentSet := make(map[string]struct{}, len(current))
	for _, c := range current {
		currentSet[c.Hostname] = struct{}{}
	}

	// Index expected routes by hostname.
	expectedSet := make(map[string]RouteConfig, len(expected))
	for _, e := range expected {
		expectedSet[e.Hostname] = e
	}

	// Add routes present in DB but missing from Caddy.
	for hostname, cfg := range expectedSet {
		if _, exists := currentSet[hostname]; !exists {
			if err := r.proxy.AddRoute(ctx, cfg); err != nil {
				slog.Warn("proxy reconciler: failed to restore route", "hostname", hostname, "error", err)
				recordErr(err)
			} else {
				slog.Info("proxy reconciler: restored missing route", "hostname", hostname)
				added++
			}
		}
	}

	// Remove routes in Caddy that no longer exist in DB.
	for hostname := range currentSet {
		if _, exists := expectedSet[hostname]; !exists {
			if err := r.proxy.RemoveRoute(ctx, hostname); err != nil {
				slog.Warn("proxy reconciler: failed to remove stale route", "hostname", hostname, "error", err)
				recordErr(err)
			} else {
				slog.Info("proxy reconciler: removed stale route", "hostname", hostname)
				removed++
			}
		}
	}

	if added > 0 || removed > 0 {
		slog.Info("proxy reconciler: reconciliation complete", "added", added, "removed", removed)
	}
}

// buildExpected returns the full set of RouteConfigs that should exist in
// Caddy, derived from all running applications and their domains in the DB.
func (r *Reconciler) buildExpected(ctx context.Context) ([]RouteConfig, error) {
	apps, err := r.queries.ListAllApplications(ctx)
	if err != nil {
		return nil, err
	}

	var configs []RouteConfig
	for _, app := range apps {
		if app.Status != status.ApplicationRunning {
			continue
		}

		row, err := r.queries.GetApplicationWithProjectSlug(ctx, app.ID)
		if err != nil {
			slog.Warn("proxy reconciler: failed to get application slug", "error", err)
			continue
		}

		// naming.ContainerName returns appSlug directly (the DB slug is the full container name).
		containerName := naming.ContainerName(row.ProjectSlug, row.Slug, "")

		domains, err := r.queries.ListDomainsByApplication(ctx, app.ID)
		if err != nil {
			slog.Warn("proxy reconciler: failed to list domains", "error", err)
			continue
		}

		for _, domain := range domains {
			cfg, err := BuildRouteConfigFromDB(ctx, r.queries, domain, containerName, app.HealthCheckPath.String)
			if err != nil {
				slog.Warn("proxy reconciler: failed to build route config", "hostname", domain.Hostname, "error", err)
				continue
			}
			configs = append(configs, cfg)
		}
	}

	return configs, nil
}
