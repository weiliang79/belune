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

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type createApplicationRequest struct {
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`             // optional, auto-generated from name if empty
	Type            string  `json:"type"`             // "git" or "image"
	SourceRepo      string  `json:"source_repo"`      // for git type
	SourceImage     string  `json:"source_image"`     // for image type
	DockerfilePath  string  `json:"dockerfile_path"`  // optional
	BuildType       string  `json:"build_type"`       // dockerfile, buildpacks, railpack, image
	CPULimit        float64 `json:"cpu_limit"`        // CPU cores (0 = unlimited)
	MemoryLimit     int64   `json:"memory_limit"`     // bytes (0 = unlimited)
	GitToken        string  `json:"git_token"`         // PAT for private repos; encrypted server-side
	HealthCheckPath string  `json:"health_check_path"` // HTTP path to poll after deploy (e.g. /healthz)
}

func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
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

	var req createApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" || req.BuildType == "" {
		writeError(w, http.StatusBadRequest, "name, type, and build_type are required")
		return
	}

	// Fetch project to get its slug for the final slug format
	project, err := h.queries.GetProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	baseSlug := naming.Slugify(req.Name)
	if req.Slug != "" {
		baseSlug = naming.Slugify(req.Slug)
	}

	app, err := h.appService.Create(r.Context(), service.CreateApplicationParams{
		ProjectID:      projectUUID,
		ProjectSlug:    project.Slug,
		Name:           req.Name,
		BaseSlug:       baseSlug,
		Type:           req.Type,
		SourceRepo:     req.SourceRepo,
		SourceImage:    req.SourceImage,
		DockerfilePath: req.DockerfilePath,
		BuildType:      req.BuildType,
		CPULimit:        req.CPULimit,
		MemoryLimit:     req.MemoryLimit,
		GitToken:        req.GitToken,
		HealthCheckPath: req.HealthCheckPath,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create application")
		return
	}

	h.audit(r, "create_application", "application", uuidToString(app.ID), map[string]any{"name": req.Name, "project_id": projectID})

	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "applicationId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	app, err := h.queries.GetApplication(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
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

	applications, err := h.queries.ListApplicationsByProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	writeJSON(w, http.StatusOK, applications)
}

type deployPayload struct {
	ApplicationID    string `json:"application_id"`
	DeploymentID     string `json:"deployment_id"`
	RollbackImageTag string `json:"rollback_image_tag,omitempty"`
}

func (h *Handler) DeployApplication(w http.ResponseWriter, r *http.Request) {
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

	// Verify application exists
	_, err := h.queries.GetApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	// Create deployment record
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ApplicationID: applicationUUID,
		Status:      status.DeploymentPending,
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Enqueue deploy task
	payload, err := json.Marshal(deployPayload{
		ApplicationID: applicationID,
		DeploymentID:  fmt.Sprintf("%x-%x-%x-%x-%x", deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6], deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16]),
	})
	if err != nil {
		slog.Error("failed to marshal deploy payload", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create deploy task")
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
		writeError(w, http.StatusInternalServerError, "failed to enqueue deploy task")
		return
	}

	h.audit(r, "deploy_application", "application", applicationID, nil)

	writeJSON(w, http.StatusAccepted, deployment)
}

func (h *Handler) StopApplication(w http.ResponseWriter, r *http.Request) {
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

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.runtime.StopContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop application")
		return
	}

	app, err := h.queries.UpdateApplicationStatus(r.Context(), generated.UpdateApplicationStatusParams{
		ID:     applicationUUID,
		Status: status.ApplicationStopped,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	h.audit(r, "stop_application", "application", applicationID, nil)

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) StartApplication(w http.ResponseWriter, r *http.Request) {
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

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		slog.Error("failed to start application container", "container", containerName, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to start application")
		return
	}

	app, err := h.queries.UpdateApplicationStatus(r.Context(), generated.UpdateApplicationStatusParams{
		ID:     applicationUUID,
		Status: status.ApplicationRunning,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	h.audit(r, "start_application", "application", applicationID, nil)

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) RestartApplication(w http.ResponseWriter, r *http.Request) {
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

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	// Stop and start the existing container (no rebuild)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.runtime.StopContainer(r.Context(), containerName); err != nil {
		slog.Error("failed to stop container for restart", "container", containerName, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to stop container")
		return
	}
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		slog.Error("failed to start container for restart", "container", containerName, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to start container")
		return
	}

	app, err := h.queries.UpdateApplicationStatus(r.Context(), generated.UpdateApplicationStatusParams{
		ID:     applicationUUID,
		Status: status.ApplicationRunning,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	h.audit(r, "restart_application", "application", applicationID, nil)

	writeJSON(w, http.StatusOK, app)
}

type updateApplicationRequest struct {
	Name              string  `json:"name"`
	SourceRepo        string  `json:"source_repo"`
	SourceImage       string  `json:"source_image"`
	DockerfilePath    string  `json:"dockerfile_path"`
	BuildTypeOverride string  `json:"build_type_override"`
	BuilderImage      string  `json:"builder_image"`
	CPULimit          float64 `json:"cpu_limit"`
	MemoryLimit       int64   `json:"memory_limit"`
	GitToken          string  `json:"git_token"`          // PAT for private repos; encrypted server-side; empty = preserve existing
	HealthCheckPath   string  `json:"health_check_path"`  // HTTP path to poll after deploy; empty = clear
}

func (h *Handler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
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

	// Get current application to preserve unchanged fields
	current, err := h.queries.GetApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	var req updateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := h.appService.Update(r.Context(), applicationUUID, current, service.UpdateApplicationParams{
		Name:              req.Name,
		SourceRepo:        req.SourceRepo,
		SourceImage:       req.SourceImage,
		DockerfilePath:    req.DockerfilePath,
		BuildTypeOverride: req.BuildTypeOverride,
		BuilderImage:      req.BuilderImage,
		CPULimit:          req.CPULimit,
		MemoryLimit:       req.MemoryLimit,
		GitToken:          req.GitToken,
		HealthCheckPath:   req.HealthCheckPath,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application")
		return
	}

	h.audit(r, "update_application", "application", applicationID, nil)

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
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

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	if err := h.appService.Delete(r.Context(), applicationUUID, row.ProjectSlug, row.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}

	h.audit(r, "delete_application", "application", applicationID, map[string]any{"name": row.Name})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) BuildApplication(w http.ResponseWriter, r *http.Request) {
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

	// Verify application exists
	_, err := h.queries.GetApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	// Create deployment record
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ApplicationID: applicationUUID,
		Status:      status.DeploymentPending,
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Enqueue build task (not deploy)
	payload, _ := json.Marshal(deployPayload{
		ApplicationID: applicationID,
		DeploymentID:  fmt.Sprintf("%x-%x-%x-%x-%x", deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6], deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16]),
	})

	task := asynq.NewTask("build", payload)
	_, err = h.asynq.Enqueue(task,
		asynq.Queue("default"),
		asynq.Timeout(time.Duration(h.cfg.TaskTimeoutMinutes)*time.Minute),
		asynq.MaxRetry(3),
		asynq.TaskID("deploy:"+applicationID),
	)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			writeError(w, http.StatusConflict, "a deployment is already in progress for this application")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to enqueue build task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}
