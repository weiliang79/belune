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

	"github.com/weiling79/belune/internal/pkg/tracing"
	"github.com/weiling79/belune/internal/server/middleware"
	"github.com/weiling79/belune/internal/status"
	"github.com/weiling79/belune/internal/store/generated"
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

// GetGlobalDeployments returns deployments across all applications.
// Admins see all; members see only their own projects' deployments.
// GET /api/deployments
func (h *Handler) GetGlobalDeployments(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	role := middleware.RoleFromContext(r.Context())

	params := generated.ListGlobalDeploymentsFilteredParams{
		Limit:  limit,
		Offset: offset,
	}

	// Optional filters
	if v := r.URL.Query().Get("project_id"); v != "" {
		var uuid pgtype.UUID
		if err := uuid.Scan(v); err != nil {
			writeError(w, http.StatusBadRequest, "invalid project_id format")
			return
		}
		params.ProjectID = uuid
	}
	if v := r.URL.Query().Get("application_id"); v != "" {
		var uuid pgtype.UUID
		if err := uuid.Scan(v); err != nil {
			writeError(w, http.StatusBadRequest, "invalid application_id format")
			return
		}
		params.ApplicationID = uuid
	}
	if v := r.URL.Query().Get("status"); v != "" {
		params.Status = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("search"); v != "" {
		params.Search = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from format, expected RFC3339")
			return
		}
		params.From = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to format, expected RFC3339")
			return
		}
		params.To = pgtype.Timestamptz{Time: t, Valid: true}
	}

	// Non-admins are scoped to their own projects
	if role != "admin" {
		userIDStr := middleware.UserIDFromContext(r.Context())
		var userUUID pgtype.UUID
		if err := userUUID.Scan(userIDStr); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id in token")
			return
		}
		params.UserID = userUUID
	}

	rows, err := h.queries.ListGlobalDeploymentsFiltered(r.Context(), params)
	if err != nil {
		slog.Error("failed to list global deployments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}
	writeJSON(w, http.StatusOK, rows)
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
		TraceCarrier:     tracing.InjectContext(r.Context()),
	})
	if err != nil {
		slog.Error("failed to marshal rollback payload", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create rollback task")
		return
	}

	if err := h.enqueueDeployTask(applicationID, payload); err != nil {
		h.failDeploymentEnqueue(r.Context(), deployment.ID, err)
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
