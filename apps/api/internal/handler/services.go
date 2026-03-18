package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type createServiceRequest struct {
	Name           string `json:"name"`
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

	svc, err := h.queries.CreateService(r.Context(), generated.CreateServiceParams{
		ProjectID:      projectUUID,
		Name:           req.Name,
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

	containerName := fmt.Sprintf("paas-%s", serviceID[:8])
	if err := h.runtime.StopContainer(r.Context(), containerName); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop service")
		return
	}

	h.queries.UpdateServiceStatus(r.Context(), generated.UpdateServiceStatusParams{
		ID:     serviceUUID,
		Status: "stopped",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *Handler) RestartService(w http.ResponseWriter, r *http.Request) {
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

	// Stop existing container (ignore errors)
	containerName := fmt.Sprintf("paas-%s", serviceID[:8])
	_ = h.runtime.StopContainer(r.Context(), containerName)
	_ = h.runtime.RemoveContainer(r.Context(), containerName)

	// Trigger a new deploy
	deployment, err := h.queries.CreateDeployment(r.Context(), generated.CreateDeploymentParams{
		ServiceID:   serviceUUID,
		Status:      "pending",
		TriggeredBy: "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

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

	// Stop and remove the container
	containerName := fmt.Sprintf("paas-%s", serviceID[:8])
	_ = h.runtime.StopContainer(r.Context(), containerName)
	_ = h.runtime.RemoveContainer(r.Context(), containerName)

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
