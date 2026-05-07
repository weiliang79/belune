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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/tracing"
	"github.com/ungweiliang/selfhost-paas/internal/quota"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/worker"
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

	if h.quotaSvc != nil {
		if err := h.quotaSvc.CheckApplicationCreate(r.Context(), projectUUID, project.UserID, req.CPULimit, req.MemoryLimit); err != nil {
			var qe *quota.ExceededError
			if errors.As(err, &qe) {
				writeError(w, qe.HTTPStatus(), qe.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "quota check failed")
			return
		}
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

	h.maybeEnqueueQuotaAlert(r, projectUUID, project.UserID)

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

// GetApplicationHealth returns the most recent post-deploy health probe
// result. The probe runs as the final stage of HandleDeployTask and writes
// to deployments.health_status; this endpoint is a read-only view of that
// row. Status values:
//
//   - passing | failing  — deploy probe outcome
//   - skipped            — app has no health_check_path configured
//   - pending            — no deploy has run yet (no row found)
func (h *Handler) GetApplicationHealth(w http.ResponseWriter, r *http.Request) {
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

	row, err := h.queries.GetLatestApplicationHealth(r.Context(), uuid)
	if err != nil {
		// pgx.ErrNoRows here means the application has no deployments yet;
		// surface that as a "pending" state rather than a 404 so the UI can
		// distinguish "never deployed" from "app missing".
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "pending",
			"message": "no deployments yet",
		})
		return
	}

	status := row.HealthStatus.String
	if !row.HealthStatus.Valid || status == "" {
		status = "pending"
	}
	resp := map[string]any{
		"deployment_id":  row.ID,
		"deploy_status":  row.Status,
		"status":         status,
		"message":        row.HealthMessage.String,
	}
	if row.HealthCheckedAt.Valid {
		resp["checked_at"] = row.HealthCheckedAt.Time
	}
	writeJSON(w, http.StatusOK, resp)
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
	ApplicationID    string            `json:"application_id"`
	DeploymentID     string            `json:"deployment_id"`
	RollbackImageTag string            `json:"rollback_image_tag,omitempty"`
	TraceCarrier     map[string]string `json:"trace_carrier,omitempty"`
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
		TraceCarrier:  tracing.InjectContext(r.Context()),
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

// GetBuildCache reports the size (in bytes) of the CNB build + launch cache
// volumes for this application. Missing volumes contribute zero; an app that
// has never built surfaces 0 B with no error.
func (h *Handler) GetBuildCache(w http.ResponseWriter, r *http.Request) {
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

	if _, err := h.queries.GetApplication(r.Context(), applicationUUID); err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	buildVol := naming.CNBCacheVolumeName(applicationID)
	launchVol := naming.CNBLaunchCacheVolumeName(applicationID)

	sizes, err := h.runtime.VolumeSizes(r.Context(), []string{buildVol, launchVol})
	if err != nil {
		slog.Warn("failed to read cache volume sizes", "application_id", applicationID, "error", err)
	}

	buildSize := sizes[buildVol]
	launchSize := sizes[launchVol]

	writeJSON(w, http.StatusOK, map[string]any{
		"build_cache_bytes":  buildSize,
		"launch_cache_bytes": launchSize,
		"total_bytes":        buildSize + launchSize,
	})
}

// ClearBuildCache deletes the application's CNB cache volumes. The next
// build will re-create them from scratch — this is the "make it fresh"
// escape hatch when a cache is suspected of poisoning output.
func (h *Handler) ClearBuildCache(w http.ResponseWriter, r *http.Request) {
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

	if _, err := h.queries.GetApplication(r.Context(), applicationUUID); err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	for _, vol := range []string{
		naming.CNBCacheVolumeName(applicationID),
		naming.CNBLaunchCacheVolumeName(applicationID),
	} {
		if err := h.runtime.RemoveVolume(r.Context(), vol); err != nil {
			slog.Debug("could not remove cache volume (may not exist)", "volume", vol, "error", err)
		}
	}

	h.audit(r, "clear_build_cache", "application", applicationID, nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
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
		TraceCarrier:  tracing.InjectContext(r.Context()),
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

// isNotFound returns true when err is a pgx no-rows sentinel (quota prefs table miss).
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// newAlertQuotaTask builds the asynq email task for a quota threshold alert.
func newAlertQuotaTask(toEmail, projectName string, usagePct, threshold int) (*asynq.Task, error) {
	return worker.NewEmailSendTask("alert_quota_threshold", toEmail, map[string]any{
		"ProjectName":      projectName,
		"QuotaType":        "applications",
		"UsagePercent":     usagePct,
		"ThresholdPercent": threshold,
	})
}

// maybeEnqueueQuotaAlert checks the project's quota usage after an app is created
// and enqueues an alert email if the owner's threshold has been crossed.
func (h *Handler) maybeEnqueueQuotaAlert(r *http.Request, projectID, ownerUserID pgtype.UUID) {
	if h.quotaSvc == nil || h.asynq == nil {
		return
	}

	ownerPrefs, err := h.queries.GetAlertPreferences(r.Context(), ownerUserID)
	threshold := 80
	if err == nil {
		if !ownerPrefs.QuotaThreshold {
			return
		}
		threshold = int(ownerPrefs.QuotaThresholdPercent)
	} else if !isNotFound(err) {
		return
	}

	alert, err := h.quotaSvc.MaybeAlertProjectThreshold(r.Context(), projectID, threshold)
	if err != nil || alert == nil {
		return
	}

	ownerInfo, err := h.queries.GetProjectOwnerInfo(r.Context(), projectID)
	if err != nil {
		return
	}

	task, err := newAlertQuotaTask(ownerInfo.Email, ownerInfo.ProjectName, alert.CurrentPercent, alert.Threshold)
	if err != nil {
		slog.Warn("quota alert: failed to build email task", "error", err)
		return
	}
	if _, err := h.asynq.Enqueue(task); err != nil {
		slog.Warn("quota alert: failed to enqueue email task", "error", err)
	}
}
