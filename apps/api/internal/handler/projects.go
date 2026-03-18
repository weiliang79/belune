package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

type createProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))

	project, err := h.queries.CreateProject(r.Context(), generated.CreateProjectParams{
		Name:   req.Name,
		Slug:   req.Slug,
		UserID: userID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))

	projects, err := h.queries.ListProjectsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

type updateProjectRequest struct {
	Name string `json:"name"`
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	project, err := h.queries.UpdateProject(r.Context(), generated.UpdateProjectParams{
		ID:   uuid,
		Name: req.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Stop and remove all service containers for this project
	project, _ := h.queries.GetProject(r.Context(), uuid)
	services, err := h.queries.ListServicesByProject(r.Context(), uuid)
	if err == nil {
		for _, svc := range services {
			svcID := fmt.Sprintf("%x-%x-%x-%x-%x", svc.ID.Bytes[0:4], svc.ID.Bytes[4:6], svc.ID.Bytes[6:8], svc.ID.Bytes[8:10], svc.ID.Bytes[10:16])
			containerName := naming.ContainerName(project.Slug, svc.Slug, svcID)
			intermediateContainerName := naming.IntermediateContainerName(project.Slug, svcID)
			oldContainerName := naming.OldContainerName(svcID)
			_ = h.runtime.StopContainer(r.Context(), containerName)
			_ = h.runtime.RemoveContainer(r.Context(), containerName)
			_ = h.runtime.StopContainer(r.Context(), intermediateContainerName)
			_ = h.runtime.RemoveContainer(r.Context(), intermediateContainerName)
			_ = h.runtime.StopContainer(r.Context(), oldContainerName)
			_ = h.runtime.RemoveContainer(r.Context(), oldContainerName)
		}
	}

	if err := h.queries.DeleteProject(r.Context(), uuid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
