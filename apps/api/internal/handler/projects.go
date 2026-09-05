package handler

import (
	"context"
	"encoding/json"
	"log/slog"
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
	// middleware.RequireProjectAccess only ever compares against a
	// {projectId} URL param, so it has nothing to check here — creating a
	// project has no existing id to pin against, but a new project is by
	// definition not the one a pinned token was narrowed to. Reject
	// explicitly rather than let a pinned token escape its pin by creating
	// somewhere new to work.
	if middleware.TokenProjectFromContext(r.Context()) != "" {
		writeError(w, http.StatusForbidden, "token is pinned to a different project")
		return
	}

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

	// Every project is placed on a server. With no agent yet that is always the
	// control plane's own host; the caller does not get to choose.
	serverID, err := h.serverSvc.LocalServerID(r.Context())
	if err != nil {
		slog.Error("failed to resolve the local server", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	project, err := h.queries.CreateProject(r.Context(), generated.CreateProjectParams{
		Name:     req.Name,
		Slug:     req.Slug,
		UserID:   userID,
		ServerID: serverID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	h.audit(r, "create_project", "project", uuidToString(project.ID), map[string]any{"name": req.Name})

	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) projectOwner(projectID pgtype.UUID) ownerLookup {
	return func(ctx context.Context) (pgtype.UUID, bool, error) {
		project, err := h.queries.GetProject(ctx, projectID)
		return project.UserID, project.Shared, err
	}
}

// canAccessProject checks if the current user can access the given project.
// Admins can access all projects; members can access their own and any shared
// project. This is read/use access — destructive rights (delete, transfer,
// change sharing) require isProjectOwner instead.
func (h *Handler) canAccessProject(r *http.Request, projectID pgtype.UUID) bool {
	return h.canAccessOwned(r, h.projectOwner(projectID))
}

// isProjectOwner checks if the current user owns the given project. Admins
// bypass the check. Unlike canAccessProject, sharing does NOT grant this —
// delete, transfer, and changing sharing itself stay owner-only, or a shared
// member could unshare or destroy a project they do not own.
func (h *Handler) isProjectOwner(r *http.Request, projectID pgtype.UUID) bool {
	return h.isOwnerOnly(r, h.projectOwner(projectID))
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
	// This list has no {projectId} URL param for middleware.RequireProjectAccess
	// to check, so a pinned token would otherwise see every project's
	// name/slug it could enumerate before the pin — narrow the result here.
	pinned := middleware.TokenProjectFromContext(r.Context())

	if role == "admin" {
		projects, err := h.queries.ListAllProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		writeJSON(w, http.StatusOK, filterProjectsByPin(projects, pinned, func(p generated.ListAllProjectsRow) pgtype.UUID { return p.ID }))
		return
	}

	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))

	projects, err := h.queries.ListProjectsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, filterProjectsByPin(projects, pinned, func(p generated.ListProjectsByUserRow) pgtype.UUID { return p.ID }))
}

// filterProjectsByPin narrows rows to the pinned project id when pinned is
// non-empty, and returns rows unchanged (never nil) otherwise. Generic over
// the two list rows' near-identical but distinct sqlc-generated types.
func filterProjectsByPin[T any](rows []T, pinned string, id func(T) pgtype.UUID) []T {
	if pinned == "" {
		return rows
	}
	var pinnedUUID pgtype.UUID
	if err := pinnedUUID.Scan(pinned); err != nil {
		return []T{}
	}
	out := make([]T, 0, len(rows))
	for _, row := range rows {
		if id(row) == pinnedUUID {
			out = append(out, row)
		}
	}
	return out
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

	if !h.isProjectOwner(r, uuid) {
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

type updateProjectSharingRequest struct {
	Shared bool `json:"shared"`
}

// UpdateProjectSharing turns project sharing on or off. Owner or admin only —
// a Member who only has shared access must not be able to unshare or reshare
// a project they do not own.
func (h *Handler) UpdateProjectSharing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if !h.isProjectOwner(r, uuid) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateProjectSharingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := h.queries.UpdateProjectSharing(r.Context(), generated.UpdateProjectSharingParams{
		ID:     uuid,
		Shared: req.Shared,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project sharing")
		return
	}

	h.audit(r, "update_project_sharing", "project", id, map[string]any{"shared": req.Shared})

	writeJSON(w, http.StatusOK, project)
}
