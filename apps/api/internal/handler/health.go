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
	rt, err := h.runtimes.Local(ctx)
	if err != nil {
		checks["docker"] = "unhealthy: " + err.Error()
	} else if containers, err := rt.ListContainers(ctx); err != nil {
		checks["docker"] = "unhealthy: " + err.Error()
	} else {
		checks["docker"] = fmt.Sprintf("ok (%d containers)", len(containers))
	}

	// Caddy admin API — only checked when the proxy implements Ping.
	if pinger, ok := h.proxy.(interface {
		Ping(context.Context) error
	}); ok {
		if err := pinger.Ping(ctx); err != nil {
			checks["caddy"] = "unhealthy: " + err.Error()
		} else {
			checks["caddy"] = "ok"
		}
	}

	// Encryption keyring — round-trip exercise to verify ENCRYPTION_KEY_CURRENT.
	if h.cfg.Keyring != nil {
		ct, err := h.cfg.Keyring.Encrypt([]byte("healthz"))
		if err != nil {
			checks["keyring"] = "unhealthy: encrypt: " + err.Error()
		} else if pt, err := h.cfg.Keyring.Decrypt(ct); err != nil {
			checks["keyring"] = "unhealthy: decrypt: " + err.Error()
		} else if string(pt) != "healthz" {
			checks["keyring"] = "unhealthy: roundtrip mismatch"
		} else {
			checks["keyring"] = "ok"
		}
	}

	healthy := checks["database"] == "ok" &&
		(checks["redis"] == "ok" || checks["redis"] == "not configured") &&
		strings.HasPrefix(checks["docker"], "ok") &&
		(checks["caddy"] == "" || checks["caddy"] == "ok") &&
		(checks["keyring"] == "" || checks["keyring"] == "ok")

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, map[string]any{"healthy": healthy, "checks": checks})
}
