package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/crypto"
	"github.com/ungweiliang/selfhost-paas/internal/service"
)

var validGitProviders = map[string]bool{"github": true, "gitlab": true, "bitbucket": true, "gitea": true}

const (
	githubManifestStatePrefix = "gh_app_manifest:"
	manifestStateTTL          = 10 * time.Minute
)

type providerConfigResponse struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	ClientID  string `json:"client_id"`
	AppID     string `json:"app_id"`
	AppSlug   string `json:"app_slug"`
	HasSecret bool   `json:"has_secret"`
}

// ListGitProviderConfigs returns all configured provider apps (without secrets).
func (h *Handler) ListGitProviderConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.gitProviderSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list provider configs")
		return
	}
	result := make([]providerConfigResponse, 0, len(rows))
	for _, c := range rows {
		result = append(result, providerConfigResponse{
			ID:        uuidToString(c.ID),
			Provider:  c.Provider,
			BaseURL:   c.BaseUrl,
			ClientID:  c.ClientID,
			AppID:     c.AppID,
			AppSlug:   c.AppSlug,
			HasSecret: c.HasSecret.Bool,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

type saveGitProviderConfigRequest struct {
	Provider      string `json:"provider"`
	BaseURL       string `json:"base_url"`
	ClientID      string `json:"client_id"`
	AppID         string `json:"app_id"`
	AppSlug       string `json:"app_slug"`
	ClientSecret  string `json:"client_secret"`  // empty = preserve existing
	PrivateKey    string `json:"private_key"`    // GitHub App only
	WebhookSecret string `json:"webhook_secret"` // GitHub App only
}

// SaveGitProviderConfig upserts a provider app config from manual admin entry.
func (h *Handler) SaveGitProviderConfig(w http.ResponseWriter, r *http.Request) {
	var req saveGitProviderConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validGitProviders[req.Provider] {
		writeError(w, http.StatusBadRequest, "provider must be one of: github, gitlab, bitbucket, gitea")
		return
	}

	secret := h.mergeProviderSecret(r.Context(), req)

	cfg, err := h.gitProviderSvc.Save(r.Context(), service.SaveGitProviderConfigParams{
		Provider: req.Provider,
		BaseURL:  req.BaseURL,
		ClientID: req.ClientID,
		AppID:    req.AppID,
		AppSlug:  req.AppSlug,
		Secret:   secret,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save provider config")
		return
	}

	h.audit(r, "save_git_provider_config", "git_provider_config", uuidToString(cfg.ID), map[string]any{"provider": req.Provider, "base_url": req.BaseURL})
	writeJSON(w, http.StatusOK, providerConfigResponse{
		ID:        uuidToString(cfg.ID),
		Provider:  cfg.Provider,
		BaseURL:   cfg.BaseUrl,
		ClientID:  cfg.ClientID,
		AppID:     cfg.AppID,
		AppSlug:   cfg.AppSlug,
		HasSecret: len(cfg.SecretEncrypted) > 0,
	})
}

// mergeProviderSecret overlays any provided secret fields onto the existing
// stored secret so rotating one field does not wipe the others. Returns nil
// when no secret fields are provided (preserve existing).
func (h *Handler) mergeProviderSecret(ctx context.Context, req saveGitProviderConfigRequest) *service.ProviderSecret {
	if req.ClientSecret == "" && req.PrivateKey == "" && req.WebhookSecret == "" {
		return nil
	}
	merged := service.ProviderSecret{}
	if _, existing, err := h.gitProviderSvc.Get(ctx, req.Provider, req.BaseURL); err == nil {
		merged = existing
	}
	if req.ClientSecret != "" {
		merged.ClientSecret = req.ClientSecret
	}
	if req.PrivateKey != "" {
		merged.PrivateKey = req.PrivateKey
	}
	if req.WebhookSecret != "" {
		merged.WebhookSecret = req.WebhookSecret
	}
	return &merged
}

// DeleteGitProviderConfig removes a provider app config.
func (h *Handler) DeleteGitProviderConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "configId")
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid config id")
		return
	}
	if err := h.gitProviderSvc.Delete(r.Context(), uuid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete provider config")
		return
	}
	h.audit(r, "delete_git_provider_config", "git_provider_config", id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- GitHub App manifest flow ---------------------------------------------

// GetGitHubAppManifest returns a GitHub App manifest plus a one-time state nonce
// and the URL the browser should POST the manifest to. The admin's browser then
// submits the manifest form to GitHub, which creates the App and redirects to
// our public callback with a temporary code.
func (h *Handler) GetGitHubAppManifest(w http.ResponseWriter, r *http.Request) {
	state, err := crypto.GenerateWebhookSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	if err := h.rdb.Set(r.Context(), githubManifestStatePrefix+state, "1", manifestStateTTL).Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist state")
		return
	}

	base := h.cfg.PublicBaseURL
	manifest := map[string]any{
		"name":         "Self-Hosted PaaS",
		"url":          base,
		"redirect_url": base + "/api/git/providers/github/manifest/callback",
		// setup_url receives the post-install redirect (with installation_id +
		// state), which our unified connect callback turns into a connection.
		"setup_url":       base + "/api/git/integrations/callback",
		"setup_on_update": true,
		"callback_urls": []string{
			base + "/api/git/integrations/callback",
		},
		"hook_attributes": map[string]any{
			"url": base + "/api/git/webhooks/github",
		},
		"public": false,
		"default_permissions": map[string]string{
			"contents": "read",
			"metadata": "read",
		},
		"default_events": []string{"push"},
	}

	// org is optional; when set, the App is created under the organization.
	postURL := "https://github.com/settings/apps/new"
	if org := r.URL.Query().Get("org"); org != "" {
		postURL = fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", org)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"post_url": postURL,
		"state":    state,
		"manifest": manifest,
	})
}

type githubManifestConversion struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
}

// HandleGitHubAppManifestCallback is the public redirect target GitHub sends the
// admin's browser to after creating the App. It is guarded by the one-time state
// nonce (not the JWT, since a top-level redirect carries no Authorization
// header). It exchanges the code for the App credentials and stores them.
func (h *Handler) HandleGitHubAppManifestCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	// Validate and consume the one-time state.
	deleted, err := h.rdb.Del(ctx, githubManifestStatePrefix+state).Result()
	if err != nil || deleted == 0 {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}

	conv, err := convertGitHubManifest(ctx, code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to convert manifest: "+err.Error())
		return
	}

	if _, err := h.gitProviderSvc.Save(ctx, service.SaveGitProviderConfigParams{
		Provider: "github",
		BaseURL:  "",
		ClientID: conv.ClientID,
		AppID:    fmt.Sprintf("%d", conv.ID),
		AppSlug:  conv.Slug,
		Secret: &service.ProviderSecret{
			ClientSecret:  conv.ClientSecret,
			PrivateKey:    conv.PEM,
			WebhookSecret: conv.WebhookSecret,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store provider config")
		return
	}

	http.Redirect(w, r, h.cfg.PublicBaseURL+"/git-providers?connected=github", http.StatusFound)
}

// convertGitHubManifest exchanges a manifest code for App credentials.
func convertGitHubManifest(ctx context.Context, code string) (*githubManifestConversion, error) {
	url := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned status %d", resp.StatusCode)
	}

	var conv githubManifestConversion
	if err := json.NewDecoder(resp.Body).Decode(&conv); err != nil {
		return nil, fmt.Errorf("decode conversion: %w", err)
	}
	return &conv, nil
}
