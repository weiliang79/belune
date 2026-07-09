package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/pkg/buildlog"
	"github.com/weiling79/belune/internal/pkg/sse"
	"github.com/weiling79/belune/internal/status"
)

func (h *Handler) StreamBuildLogs(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	deploymentIDStr := chi.URLParam(r, "deploymentId")

	var deploymentID pgtype.UUID
	if err := deploymentID.Scan(deploymentIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}

	// Subscribe to Redis channel FIRST to avoid race condition
	sub := buildlog.NewSubscriber(h.rdb, deploymentIDStr)
	defer sub.Close()

	// Check current deployment status
	deployment, err := h.queries.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// If build is already complete, send stored logs and done event
	if deployment.Status != status.DeploymentBuilding && deployment.Status != status.DeploymentPending {
		if deployment.BuildLogs.Valid && deployment.BuildLogs.String != "" {
			for _, line := range strings.Split(deployment.BuildLogs.String, "\n") {
				if err := writer.SendData(line); err != nil {
					return
				}
			}
		}
		if deployment.ErrorMessage.Valid && deployment.ErrorMessage.String != "" {
			writer.SendEvent("error", deployment.ErrorMessage.String)
		}
		writer.SendEvent("done", fmt.Sprintf("build %s", deployment.Status))
		return
	}

	// Build is in progress — stream from Redis
	lines := sub.Channel(r.Context())
	for line := range lines {
		if err := writer.SendData(line); err != nil {
			return // Client disconnected
		}
	}

	writer.SendEvent("done", "build log stream ended")
}
