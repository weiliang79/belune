package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HealthCheck returns the health status of the application and its dependencies.
// GET /healthz
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}

	// Database
	if err := h.db.Ping(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
	} else {
		checks["database"] = "ok"
	}

	// Redis
	if h.rdb == nil {
		checks["redis"] = "not configured"
	} else if err := h.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
	} else {
		checks["redis"] = "ok"
	}

	// Docker
	containers, err := h.runtime.ListContainers(ctx)
	if err != nil {
		checks["docker"] = "unhealthy: " + err.Error()
	} else {
		checks["docker"] = fmt.Sprintf("ok (%d containers)", len(containers))
	}

	healthy := checks["database"] == "ok" &&
		(checks["redis"] == "ok" || checks["redis"] == "not configured") &&
		strings.HasPrefix(checks["docker"], "ok")

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, map[string]any{"healthy": healthy, "checks": checks})
}
