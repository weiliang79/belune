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

type createServiceRequest struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`            // optional, auto-generated from name if empty
	Type           string `json:"type"`            // "git" or "image"
	SourceRepo     string `json:"source_repo"`     // for git type
	SourceImage    string `json:"source_image"`    // for image type
	DockerfilePath string `json:"dockerfile_path"` // optional
	BuildType      string `json:"build_type"`      // dockerfile, buildpacks, railpack, image
}

func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req createServiceRequest
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

	svc, err := h.queries.CreateService(r.Context(), generated.CreateServiceParams{
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
		writeError(w, http.StatusInternalServerError, "failed to create service")
		return
	}

	// Construct final slug: {projectSlug}-{baseSlug}-{shortId}
	svcID := fmt.Sprintf("%x-%x-%x-%x-%x",
		svc.ID.Bytes[0:4], svc.ID.Bytes[4:6], svc.ID.Bytes[6:8], svc.ID.Bytes[8:10], svc.ID.Bytes[10:16])
	finalSlug := fmt.Sprintf("%s-%s-%s", project.Slug, baseSlug, svcID[:8])
	_ = h.queries.UpdateServiceSlug(r.Context(), generated.UpdateServiceSlugParams{
		ID:   svc.ID,
		Slug: finalSlug,
	})
	svc.Slug = finalSlug

	writeJSON(w, http.StatusCreated, svc)
}

func (h *Handler) GetService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serviceId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	svc, err := h.queries.GetService(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(projectID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	services, err := h.queries.ListServicesByProject(r.Context(), projectUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list services")
		return
	}

	writeJSON(w, http.StatusOK, services)
}

type deployPayload struct {
	ServiceID    string `json:"service_id"`
	DeploymentID string `json:"deployment_id"`
}

func (h *Handler) DeployService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	// Verify service exists
	_, err := h.queries.GetService(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	// Create deployment record
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ServiceID:   serviceUUID,
		Status:      "pending",
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Enqueue deploy task
	payload, _ := json.Marshal(deployPayload{
		ServiceID:    serviceID,
		DeploymentID: fmt.Sprintf("%x-%x-%x-%x-%x", deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6], deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16]),
	})

	task := asynq.NewTask("deploy", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue deploy task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}

func (h *Handler) StopService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	row, err := h.queries.GetServiceWithProjectSlug(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceID)
	if err := h.runtime.StopContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop service")
		return
	}

	svc, err := h.queries.UpdateServiceStatus(r.Context(), generated.UpdateServiceStatusParams{
		ID:     serviceUUID,
		Status: "stopped",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update service status")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (h *Handler) StartService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	row, err := h.queries.GetServiceWithProjectSlug(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceID)
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start service: "+err.Error())
		return
	}

	svc, err := h.queries.UpdateServiceStatus(r.Context(), generated.UpdateServiceStatusParams{
		ID:     serviceUUID,
		Status: "running",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update service status")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (h *Handler) RestartService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	row, err := h.queries.GetServiceWithProjectSlug(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	// Stop and start the existing container (no rebuild)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceID)
	if err := h.runtime.StopContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop container: "+err.Error())
		return
	}
	if err := h.runtime.StartContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start container: "+err.Error())
		return
	}

	svc, err := h.queries.UpdateServiceStatus(r.Context(), generated.UpdateServiceStatusParams{
		ID:     serviceUUID,
		Status: "running",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update service status")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

type updateServiceRequest struct {
	Name              string `json:"name"`
	SourceRepo        string `json:"source_repo"`
	SourceImage       string `json:"source_image"`
	DockerfilePath    string `json:"dockerfile_path"`
	BuildTypeOverride string `json:"build_type_override"`
	BuilderImage      string `json:"builder_image"`
}

func (h *Handler) UpdateService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	// Get current service to preserve unchanged fields
	current, err := h.queries.GetService(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	var req updateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := current.Name
	if req.Name != "" {
		name = req.Name
	}

	svc, err := h.queries.UpdateService(r.Context(), generated.UpdateServiceParams{
		ID:                serviceUUID,
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
		writeError(w, http.StatusInternalServerError, "failed to update service")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (h *Handler) DeleteService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	// Stop and remove the container (try all naming formats for compatibility)
	row, _ := h.queries.GetServiceWithProjectSlug(r.Context(), serviceUUID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, serviceID)
	intermediateContainerName := naming.IntermediateContainerName(row.ProjectSlug, serviceID)
	oldContainerName := naming.OldContainerName(serviceID)
	_ = h.runtime.StopContainer(r.Context(), containerName)
	_ = h.runtime.RemoveContainer(r.Context(), containerName)
	_ = h.runtime.StopContainer(r.Context(), intermediateContainerName)
	_ = h.runtime.RemoveContainer(r.Context(), intermediateContainerName)
	_ = h.runtime.StopContainer(r.Context(), oldContainerName)
	_ = h.runtime.RemoveContainer(r.Context(), oldContainerName)

	if err := h.queries.DeleteService(r.Context(), serviceUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete service")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) BuildService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceId")
	var serviceUUID pgtype.UUID
	if err := serviceUUID.Scan(serviceID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	// Verify service exists
	_, err := h.queries.GetService(r.Context(), serviceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	// Create deployment record
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ServiceID:   serviceUUID,
		Status:      "pending",
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Enqueue build task (not deploy)
	payload, _ := json.Marshal(deployPayload{
		ServiceID:    serviceID,
		DeploymentID: fmt.Sprintf("%x-%x-%x-%x-%x", deployment.ID.Bytes[0:4], deployment.ID.Bytes[4:6], deployment.ID.Bytes[6:8], deployment.ID.Bytes[8:10], deployment.ID.Bytes[10:16]),
	})

	task := asynq.NewTask("build", payload)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("default")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue build task")
		return
	}

	writeJSON(w, http.StatusAccepted, deployment)
}
