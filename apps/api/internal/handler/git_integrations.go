package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
)

const (
	gitConnectStatePrefix = "git_connect:"
	connectStateTTL       = 10 * time.Minute
)

type connectState struct {
	UserID   string `json:"user_id"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
}

type gitIntegrationResponse struct {
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"base_url"`
	AccountLogin string `json:"account_login"`
	CreatedAt    string `json:"created_at"`
}

// ListGitIntegrations returns the current user's connected provider accounts.
func (h *Handler) ListGitIntegrations(w http.ResponseWriter, r *http.Request) {
	var userID pgtype.UUID
	if err := userID.Scan(middleware.UserIDFromContext(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}
	rows, err := h.gitIntegrationSvc.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list connections")
		return
	}
	result := make([]gitIntegrationResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, gitIntegrationResponse{
			ID:           uuidToString(row.ID),
			Provider:     row.Provider,
			BaseURL:      row.BaseUrl,
			AccountLogin: row.AccountLogin,
			CreatedAt:    row.CreatedAt.Time.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

type availableProviderResponse struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	AppSlug  string `json:"app_slug"`
}

// ListAvailableProviders returns the configured providers a user can connect to.
func (h *Handler) ListAvailableProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.gitProviderSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list providers")
		return
	}
	result := make([]availableProviderResponse, 0, len(rows))
	for _, c := range rows {
		if !c.HasSecret.Bool {
			continue
		}
		result = append(result, availableProviderResponse{
			Provider: c.Provider,
			BaseURL:  c.BaseUrl,
			AppSlug:  c.AppSlug,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

// StartGitIntegrationConnect generates a one-time state, stores the connect
// context, and returns the provider auth URL for the browser to navigate to.
func (h *Handler) StartGitIntegrationConnect(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	baseURL := r.URL.Query().Get("base_url")
	if !validGitProviders[provider] {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}

	state, err := crypto.GenerateWebhookSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	payload, _ := json.Marshal(connectState{
		UserID:   middleware.UserIDFromContext(r.Context()),
		Provider: provider,
		BaseURL:  baseURL,
	})
	if err := h.rdb.Set(r.Context(), gitConnectStatePrefix+state, payload, connectStateTTL).Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist state")
		return
	}

	redirectURI := h.cfg.PublicBaseURL + "/api/git/integrations/callback"
	authURL, err := h.gitIntegrationSvc.AuthURL(r.Context(), provider, baseURL, redirectURI, state)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

// HandleGitIntegrationCallback is the public provider redirect target. It is
// guarded by the one-time state nonce (a top-level redirect carries no
// Authorization header), completes the connect flow, and redirects to the UI.
func (h *Handler) HandleGitIntegrationCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := r.URL.Query().Get("state")
	if state == "" {
		writeError(w, http.StatusBadRequest, "missing state")
		return
	}

	raw, err := h.rdb.GetDel(ctx, gitConnectStatePrefix+state).Result()
	if err != nil || raw == "" {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	var st connectState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		writeError(w, http.StatusInternalServerError, "corrupt state")
		return
	}
	var userID pgtype.UUID
	if err := userID.Scan(st.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	redirectURI := h.cfg.PublicBaseURL + "/api/git/integrations/callback"
	if _, err := h.gitIntegrationSvc.Connect(ctx, st.Provider, st.BaseURL, redirectURI, r.URL.Query(), userID); err != nil {
		http.Redirect(w, r, h.cfg.PublicBaseURL+"/git-connections?error="+st.Provider, http.StatusFound)
		return
	}
	http.Redirect(w, r, h.cfg.PublicBaseURL+"/git-connections?connected="+st.Provider, http.StatusFound)
}

// DeleteGitIntegration disconnects a connected account.
func (h *Handler) DeleteGitIntegration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "integrationId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid integration id")
		return
	}
	// Ownership: only the creator (or an admin) may disconnect.
	var userID pgtype.UUID
	_ = userID.Scan(middleware.UserIDFromContext(r.Context()))
	if err := h.gitIntegrationSvc.Delete(r.Context(), uuid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete connection")
		return
	}
	h.audit(r, "delete_git_integration", "git_integration", id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
