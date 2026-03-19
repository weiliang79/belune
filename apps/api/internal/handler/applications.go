package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type createApplicationRequest struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`            // optional, auto-generated from name if empty
	Type           string `json:"type"`            // "git" or "image"
	SourceRepo     string `json:"source_repo"`     // for git type
	SourceImage    string `json:"source_image"`    // for image type
	DockerfilePath string `json:"dockerfile_path"` // optional
	BuildType      string `json:"build_type"`      // dockerfile, buildpacks, railpack, image
}

func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
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

	app, err := h.queries.CreateApplication(r.Context(), generated.CreateApplicationParams{
		ProjectID:      projectUUID,
		Name:           req.Name,
		Slug:           baseSlug,
		Type:           req.Type,
		SourceRepo:     pgtype.Text{String: req.SourceRepo, Valid: req.SourceRepo != ""},
		SourceImage:    pgtype.Text{String: req.SourceImage, Valid: req.SourceImage != ""},
		DockerfilePath: pgtype.Text{String: req.DockerfilePath, Valid: req.DockerfilePath != ""},
		BuildType:      req.BuildType,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create application")
		return
	}

	// Construct final slug: {projectSlug}-{baseSlug}-{shortId}
	appID := fmt.Sprintf("%x-%x-%x-%x-%x",
		app.ID.Bytes[0:4], app.ID.Bytes[4:6], app.ID.Bytes[6:8], app.ID.Bytes[8:10], app.ID.Bytes[10:16])
	finalSlug := fmt.Sprintf("%s-%s-%s", project.Slug, baseSlug, appID[:8])
	_ = h.queries.UpdateApplicationSlug(r.Context(), generated.UpdateApplicationSlugParams{
		ID:   app.ID,
		Slug: finalSlug,
	})
	app.Slug = finalSlug

	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "applicationId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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

	applications, err := h.queries.ListApplicationsByProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	writeJSON(w, http.StatusOK, applications)
}

type deployPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
}

func (h *Handler) DeployApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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
		Status:      "pending",
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Enqueue deploy task
	payload, _ := json.Marshal(deployPayload{
		ApplicationID: applicationID,
		DeploymentID:  fmt.Sprintf("%x-%x-%x-%x-%x", deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6], deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16]),
	})

	task := asynq.NewTask("deploy", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue deploy task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}

func (h *Handler) StopApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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
		Status: "stopped",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) StartApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start application: "+err.Error())
		return
	}

	app, err := h.queries.UpdateApplicationStatus(r.Context(), generated.UpdateApplicationStatusParams{
		ID:     applicationUUID,
		Status: "running",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) RestartApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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
		writeError(w, http.StatusInternalServerError, "failed to stop container: "+err.Error())
		return
	}
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start container: "+err.Error())
		return
	}

	app, err := h.queries.UpdateApplicationStatus(r.Context(), generated.UpdateApplicationStatusParams{
		ID:     applicationUUID,
		Status: "running",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application status")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

type updateApplicationRequest struct {
	Name              string `json:"name"`
	SourceRepo        string `json:"source_repo"`
	SourceImage       string `json:"source_image"`
	DockerfilePath    string `json:"dockerfile_path"`
	BuildTypeOverride string `json:"build_type_override"`
	BuilderImage      string `json:"builder_image"`
}

func (h *Handler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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

	name := current.Name
	if req.Name != "" {
		name = req.Name
	}

	app, err := h.queries.UpdateApplication(r.Context(), generated.UpdateApplicationParams{
		ID:                applicationUUID,
		Name:              name,
		SourceRepo:        pgtype.Text{String: req.SourceRepo, Valid: req.SourceRepo != ""},
		SourceImage:       pgtype.Text{String: req.SourceImage, Valid: req.SourceImage != ""},
		DockerfilePath:    pgtype.Text{String: req.DockerfilePath, Valid: req.DockerfilePath != ""},
		BuildTypeOverride: pgtype.Text{String: req.BuildTypeOverride, Valid: req.BuildTypeOverride != ""},
		BuilderImage:      pgtype.Text{String: req.BuilderImage, Valid: req.BuilderImage != ""},
		CustomBuildpacks:  current.CustomBuildpacks,
		Status:            current.Status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update application")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	// Stop and remove the container (try all naming formats for compatibility)
	row, _ := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	intermediateContainerName := naming.IntermediateContainerName(row.ProjectSlug, applicationID)
	oldContainerName := naming.OldContainerName(applicationID)
	_ = h.runtime.StopContainer(r.Context(), containerName)
	_ = h.runtime.RemoveContainer(r.Context(), containerName)
	_ = h.runtime.StopContainer(r.Context(), intermediateContainerName)
	_ = h.runtime.RemoveContainer(r.Context(), intermediateContainerName)
	_ = h.runtime.StopContainer(r.Context(), oldContainerName)
	_ = h.runtime.RemoveContainer(r.Context(), oldContainerName)

	if err := h.queries.DeleteApplication(r.Context(), applicationUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete application")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) BuildApplication(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
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
		Status:      "pending",
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
	if _, err := h.asynq.Enqueue(task, asynq.Queue("default")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue build task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}
