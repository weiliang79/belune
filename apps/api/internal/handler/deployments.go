package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
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

	deployments, err := h.queries.ListDeploymentsByApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}

	writeJSON(w, http.StatusOK, deployments)
}

func (h *Handler) GetDeployment(w http.ResponseWriter, r *http.Request) {
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

	id := chi.URLParam(r, "deploymentId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}

	deployment, err := h.queries.GetDeployment(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSON(w, http.StatusOK, deployment)
}

type rollbackRequest struct {
	DeploymentID string `json:"deployment_id"`
}

// RollbackDeployment creates a new deployment using the image stored from a previous
// successful deployment, skipping the build step entirely.
// POST /api/projects/{projectId}/applications/{applicationId}/rollback
func (h *Handler) RollbackDeployment(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var appUUID pgtype.UUID
	if err := appUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, appUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DeploymentID == "" {
		writeError(w, http.StatusBadRequest, "deployment_id is required")
		return
	}

	var targetID pgtype.UUID
	if err := targetID.Scan(req.DeploymentID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}

	// Fetch the target deployment to validate it.
	target, err := h.queries.GetDeployment(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if target.ApplicationID != appUUID {
		writeError(w, http.StatusBadRequest, "deployment does not belong to this application")
		return
	}
	if target.Status != status.DeploymentSuccess {
		writeError(w, http.StatusBadRequest, "can only rollback to a successful deployment")
		return
	}
	if !target.ImageTag.Valid || target.ImageTag.String == "" {
		writeError(w, http.StatusBadRequest, "deployment has no stored image tag; rollback is unavailable")
		return
	}

	// Create a new deployment record for the rollback.
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ApplicationID: appUUID,
		Status:        status.DeploymentPending,
		TriggeredBy:   "rollback",
	})
	if err != nil {
		slog.Error("failed to create rollback deployment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	deploymentIDStr := fmt.Sprintf("%x-%x-%x-%x-%x",
		deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6],
		deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10],
		deployment.ID.Bytes[10:16])

	payload, err := json.Marshal(deployPayload{
		ApplicationID:    applicationID,
		DeploymentID:     deploymentIDStr,
		RollbackImageTag: target.ImageTag.String,
	})
	if err != nil {
		slog.Error("failed to marshal rollback payload", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create rollback task")
		return
	}

	task := asynq.NewTask("deploy", payload)
	_, err = h.asynq.Enqueue(task,
		asynq.Queue("critical"),
		asynq.Timeout(time.Duration(h.cfg.TaskTimeoutMinutes)*time.Minute),
		asynq.MaxRetry(3),
		asynq.TaskID("deploy:"+applicationID),
	)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			writeError(w, http.StatusConflict, "a deployment is already in progress for this application")
			return
		}
		slog.Error("failed to enqueue rollback task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to enqueue rollback task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}
