package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
)

// latestHostStats returns the cached host snapshot written by the metrics
// ticker (~1s fresh), falling back to a live collection when the cache is empty
// (e.g. the ticker isn't running). Reading the cache avoids re-running gopsutil
// per request and the CPU-percent jitter that a second concurrent sampler causes.
func (h *Handler) latestHostStats(ctx context.Context) service.HostMetricPoint {
	if h.rdb != nil {
		if data, err := h.rdb.Get(ctx, service.HostMetricsLatestKey).Bytes(); err == nil {
			var p service.HostMetricPoint
			if json.Unmarshal(data, &p) == nil {
				return p
			}
		}
	}
	return service.CollectHostStats(ctx)
}

type healthRatio struct {
	Running int64 `json:"running"`
	Total   int64 `json:"total"`
}

type deploy7dStats struct {
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
	Total     int64 `json:"total"`
}

type needsAttention struct {
	FailedDeploys int64 `json:"failed_deploys"`
	ErrorServices int64 `json:"error_services"`
	FailedBackups int64 `json:"failed_backups"`
	Total         int64 `json:"total"`
}

type statsResponse struct {
	IsAdmin        bool                     `json:"is_admin"`
	AppHealth      healthRatio              `json:"app_health"`
	Deploy7d       deploy7dStats            `json:"deploy_7d"`
	NeedsAttention needsAttention           `json:"needs_attention"`
	Host           *service.HostMetricPoint `json:"host"`
}

// GetStats returns the operator-health stat strip in a single call.
//
// Audience split: admins see all projects plus host resources and failed
// backups; members see only their own projects' app health, deploy success,
// and attention items (host + backups are admin-only signals).
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	isAdmin := middleware.RoleFromContext(ctx) == "admin"

	// scope is left invalid (SQL NULL) for admins so the queries count every
	// project; for members it pins aggregates to their owned projects.
	var scope pgtype.UUID
	if !isAdmin {
		if err := scope.Scan(middleware.UserIDFromContext(ctx)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid user id")
			return
		}
	}

	appH, err := h.queries.CountApplicationHealth(ctx, scope)
	if err != nil {
		slog.Warn("stats: count application health", "error", err)
	}
	dbH, err := h.queries.CountDatabaseHealth(ctx, scope)
	if err != nil {
		slog.Warn("stats: count database health", "error", err)
	}
	dep, err := h.queries.CountDeployments7d(ctx, scope)
	if err != nil {
		slog.Warn("stats: count deployments 7d", "error", err)
	}

	errorServices := appH.Errored + dbH.Errored

	resp := statsResponse{
		IsAdmin: isAdmin,
		AppHealth: healthRatio{
			Running: appH.Running + dbH.Running,
			Total:   appH.Total + dbH.Total,
		},
		Deploy7d: deploy7dStats{
			Succeeded: dep.Succeeded,
			Failed:    dep.Failed,
			Total:     dep.Total,
		},
	}

	var failedBackups int64
	if isAdmin {
		if failedBackups, err = h.queries.CountFailedBackups7d(ctx); err != nil {
			slog.Warn("stats: count failed backups", "error", err)
			failedBackups = 0
		}
		host := h.latestHostStats(ctx)
		resp.Host = &host
	}

	resp.NeedsAttention = needsAttention{
		FailedDeploys: dep.Failed,
		ErrorServices: errorServices,
		FailedBackups: failedBackups,
		Total:         dep.Failed + errorServices + failedBackups,
	}

	writeJSON(w, http.StatusOK, resp)
}
