package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

	// TODO: get user_id from auth context; using a placeholder for now
	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000000")

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
	// TODO: get user_id from auth context
	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000000")

	projects, err := h.queries.ListProjectsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
