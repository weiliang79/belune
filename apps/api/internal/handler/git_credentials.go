package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
)

type createGitCredentialRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"` // github, gitlab, bitbucket, generic
	Token    string `json:"token"`
	Username string `json:"username"`
}

func (h *Handler) ListGitCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.gitCredSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list git credentials")
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

func (h *Handler) CreateGitCredential(w http.ResponseWriter, r *http.Request) {
	var req createGitCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Provider == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "name, provider, and token are required")
		return
	}

	validProviders := map[string]bool{"github": true, "gitlab": true, "bitbucket": true, "generic": true}
	if !validProviders[req.Provider] {
		writeError(w, http.StatusBadRequest, "provider must be one of: github, gitlab, bitbucket, generic")
		return
	}

	var userUUID pgtype.UUID
	if err := userUUID.Scan(middleware.UserIDFromContext(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	cred, err := h.gitCredSvc.Create(r.Context(), service.CreateGitCredentialParams{
		Name:      req.Name,
		Provider:  req.Provider,
		Token:     req.Token,
		Username:  req.Username,
		CreatedBy: userUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create git credential")
		return
	}

	h.audit(r, "create_git_credential", "git_credential", uuidToString(cred.ID), map[string]any{"name": req.Name, "provider": req.Provider})

	writeJSON(w, http.StatusCreated, cred)
}

func (h *Handler) UpdateGitCredential(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credentialId")
	var credUUID pgtype.UUID
	if err := credUUID.Scan(credID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	var req createGitCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Provider == "" {
		writeError(w, http.StatusBadRequest, "name and provider are required")
		return
	}

	cred, err := h.gitCredSvc.Update(r.Context(), credUUID, service.UpdateGitCredentialParams{
		Name:     req.Name,
		Provider: req.Provider,
		Token:    req.Token, // empty = preserve existing
		Username: req.Username,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update git credential")
		return
	}

	h.audit(r, "update_git_credential", "git_credential", credID, nil)

	writeJSON(w, http.StatusOK, cred)
}

func (h *Handler) DeleteGitCredential(w http.ResponseWriter, r *http.Request) {
	credID := chi.URLParam(r, "credentialId")
	var credUUID pgtype.UUID
	if err := credUUID.Scan(credID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	if err := h.gitCredSvc.Delete(r.Context(), credUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete git credential")
		return
	}

	h.audit(r, "delete_git_credential", "git_credential", credID, nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
