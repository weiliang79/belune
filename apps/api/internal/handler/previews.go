package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/store/generated"
)

type previewConfigRequest struct {
	PreviewBranchPattern  *string `json:"preview_branch_pattern"`
	PreviewDomainTemplate *string `json:"preview_domain_template"`
}

// UpdatePreviewConfig sets or clears the parent app's preview config. A nil
// field leaves the stored value untouched; an empty string disables that side.
// Disabling both effectively turns previews off for this parent (future pushes
// that don't match auto_deploy_branch are ignored).
func (h *Handler) UpdatePreviewConfig(w http.ResponseWriter, r *http.Request) {
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

	current, err := h.queries.GetApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	if current.ParentApplicationID.Valid {
		writeError(w, http.StatusBadRequest, "preview apps cannot host nested previews")
		return
	}

	var req previewConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pattern := current.PreviewBranchPattern
	if req.PreviewBranchPattern != nil {
		v := strings.TrimSpace(*req.PreviewBranchPattern)
		pattern = pgtype.Text{String: v, Valid: v != ""}
	}

	template := current.PreviewDomainTemplate
	if req.PreviewDomainTemplate != nil {
		v := strings.TrimSpace(*req.PreviewDomainTemplate)
		if v != "" && !strings.Contains(v, "{branch}") {
			writeError(w, http.StatusBadRequest, "preview_domain_template must contain {branch}")
			return
		}
		template = pgtype.Text{String: v, Valid: v != ""}
	}

	row, err := h.queries.UpdateApplicationPreviewConfig(r.Context(), generated.UpdateApplicationPreviewConfigParams{
		ID:                    applicationUUID,
		PreviewBranchPattern:  pattern,
		PreviewDomainTemplate: template,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update preview config")
		return
	}
	h.audit(r, "update_preview_config", "application", applicationID, map[string]any{
		"pattern":  pattern.String,
		"template": template.String,
	})
	writeJSON(w, http.StatusOK, previewConfigView(row))
}

type previewView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Branch         string `json:"branch"`
	Status         string `json:"status"`
	LastActivityAt string `json:"last_activity_at"`
	Hostname       string `json:"hostname,omitempty"`
}

// ListPreviews returns all preview children of the given parent application.
func (h *Handler) ListPreviews(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.queries.ListPreviewsByParent(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list previews")
		return
	}

	out := make([]previewView, 0, len(rows))
	for _, row := range rows {
		view := previewView{
			ID:     uuidToString(row.ID),
			Name:   row.Name,
			Slug:   row.Slug,
			Branch: row.Branch.String,
			Status: row.Status,
		}
		if row.LastActivityAt.Valid {
			view.LastActivityAt = row.LastActivityAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		}
		if domains, derr := h.queries.ListDomainsByApplication(r.Context(), row.ID); derr == nil && len(domains) > 0 {
			view.Hostname = domains[0].Hostname
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"previews": out})
}

// DeletePreview removes a single preview child. Same semantics as
// DeleteApplication except callers cannot use this path on a non-preview.
func (h *Handler) DeletePreview(w http.ResponseWriter, r *http.Request) {
	previewID := chi.URLParam(r, "previewId")
	var previewUUID pgtype.UUID
	if err := previewUUID.Scan(previewID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid preview id")
		return
	}
	if !h.canAccessApplication(r, previewUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), previewUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "preview not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch preview")
		return
	}
	if !row.ParentApplicationID.Valid {
		writeError(w, http.StatusBadRequest, "not a preview application")
		return
	}

	if err := h.appService.Delete(r.Context(), previewUUID, row.ProjectSlug, row.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete preview")
		return
	}
	h.audit(r, "delete_preview", "application", previewID, map[string]any{"branch": row.Branch.String})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type previewConfigResponse struct {
	PreviewBranchPattern  string `json:"preview_branch_pattern"`
	PreviewDomainTemplate string `json:"preview_domain_template"`
}

func previewConfigView(app generated.Application) previewConfigResponse {
	return previewConfigResponse{
		PreviewBranchPattern:  app.PreviewBranchPattern.String,
		PreviewDomainTemplate: app.PreviewDomainTemplate.String,
	}
}
