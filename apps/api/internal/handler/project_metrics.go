package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/runtime"
)

// serviceMetrics is a per-service runtime snapshot for the project overview.
type serviceMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryLimit   int64   `json:"memory_limit"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Status        string  `json:"status"`
	Domain        string  `json:"domain,omitempty"`
	Port          int32   `json:"port,omitempty"`
}

// GetProjectMetrics returns a current runtime snapshot (CPU%, memory, uptime,
// status, primary domain/port) for each application in a project, keyed by
// application id. Container stats are a best-effort single poll — apps without a
// running container simply omit live usage. GET /api/projects/{projectId}/metrics
func (h *Handler) GetProjectMetrics(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if !h.canAccessProject(r, projectUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	ctx := r.Context()

	apps, err := h.queries.ListApplicationsByProject(ctx, projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	// Primary domain/port per app (best-effort; ignore on error).
	domainByApp := map[string]struct {
		hostname string
		port     int32
	}{}
	if rows, derr := h.queries.ListProjectAppPrimaryDomain(ctx, projectUUID); derr == nil {
		for _, row := range rows {
			domainByApp[uuidToString(row.ApplicationID)] = struct {
				hostname string
				port     int32
			}{row.Hostname.String, row.ContainerPort.Int32}
		}
	}

	// One container listing, indexed by name (== app slug).
	containerByName := map[string]runtime.ContainerInfo{}
	if containers, cerr := h.runtime.ListContainers(ctx); cerr == nil {
		for _, c := range containers {
			containerByName[c.Name] = c
		}
	}

	result := map[string]serviceMetrics{}
	for _, app := range apps {
		appID := uuidToString(app.ID)
		m := serviceMetrics{Status: app.Status, MemoryLimit: app.MemoryLimit}

		if c, ok := containerByName[app.Slug]; ok {
			m.Status = c.Status
			if !c.CreatedAt.IsZero() {
				m.UptimeSeconds = int64(time.Since(c.CreatedAt).Seconds())
			}
			if c.Status == "running" {
				if s, serr := h.runtime.ContainerStats(ctx, app.Slug); serr == nil {
					m.CPUPercent = s.CPUPercent
					m.MemoryUsed = s.MemoryUsage
					if s.MemoryLimit > 0 {
						m.MemoryLimit = s.MemoryLimit
					}
				}
			}
		}

		if d, ok := domainByApp[appID]; ok {
			m.Domain = d.hostname
			m.Port = d.port
		}
		result[appID] = m
	}

	writeJSON(w, http.StatusOK, result)
}
