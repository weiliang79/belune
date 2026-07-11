package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/store/generated"
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

	h.audit(r, "create_project", "project", uuidToString(project.ID), map[string]any{"name": req.Name})

	writeJSON(w, http.StatusCreated, project)
}

// canAccessProject checks if the current user can access the given project.
// Admins can access all projects; members can only access their own.
func (h *Handler) canAccessProject(r *http.Request, projectID pgtype.UUID) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return project.UserID == userID
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if !h.canAccessProject(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
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
	role := middleware.RoleFromContext(r.Context())

	if role == "admin" {
		projects, err := h.queries.ListAllProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		writeJSON(w, http.StatusOK, projects)
		return
	}

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

	if !h.canAccessProject(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
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

	h.audit(r, "update_project", "project", id, nil)

	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if !h.canAccessProject(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.projService.Delete(r.Context(), uuid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	h.audit(r, "delete_project", "project", id, nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type transferProjectRequest struct {
	UserID string `json:"user_id"`
}

func (h *Handler) TransferProject(w http.ResponseWriter, r *http.Request) {
	// Admin-only operation
	role := middleware.RoleFromContext(r.Context())
	if role != "admin" {
		writeError(w, http.StatusForbidden, "only admins can transfer projects")
		return
	}

	id := chi.URLParam(r, "projectId")
	var projectUUID pgtype.UUID
	if err := projectUUID.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req transferProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	// Verify target user exists
	var newOwnerID pgtype.UUID
	if err := newOwnerID.Scan(req.UserID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if _, err := h.queries.GetUserByID(r.Context(), newOwnerID); err != nil {
		writeError(w, http.StatusNotFound, "target user not found")
		return
	}

	project, err := h.queries.UpdateProjectOwner(r.Context(), generated.UpdateProjectOwnerParams{
		ID:     projectUUID,
		UserID: newOwnerID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to transfer project")
		return
	}

	writeJSON(w, http.StatusOK, project)
}
