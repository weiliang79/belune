package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/naming"
	"github.com/weiling79/belune/internal/proxy"
	"github.com/weiling79/belune/internal/store/generated"
)

// hostnameRegex validates RFC 1123 hostnames. Each label is 1–63 alphanumeric characters
// or internal hyphens; the full name must have at least one dot (or be "localhost").
var hostnameRegex = regexp.MustCompile(`^(([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}|localhost)$`)

// uuidToString converts a pgtype.UUID to its string representation.
func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

var validFeatureTypes = map[string]bool{
	"basic_auth":   true,
	"redirect":     true,
	"headers":      true,
	"ip_allowlist": true,
	"rate_limit":   true,
}

var validSSLModes = map[string]bool{
	"automatic":     true,
	"dns_challenge": true,
	"custom":        true,
	"off":           true,
}

func (h *Handler) ListDomains(w http.ResponseWriter, r *http.Request) {
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

	domains, err := h.queries.ListDomainsByApplicationWithFeatures(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list domains")
		return
	}

	writeJSON(w, http.StatusOK, domains)
}

type addDomainRequest struct {
	Hostname      string          `json:"hostname"`
	SSLEnabled    bool            `json:"ssl_enabled"`
	ContainerPort *int32          `json:"container_port,omitempty"`
	ForceHTTPS    *bool           `json:"force_https,omitempty"`
	SSLMode       string          `json:"ssl_mode,omitempty"`
	SSLProvider   string          `json:"ssl_provider,omitempty"`
	CertPath      string          `json:"cert_path,omitempty"`
	KeyPath       string          `json:"key_path,omitempty"`
	AdvancedConfig json.RawMessage `json:"advanced_config,omitempty"`
}

func (h *Handler) AddDomain(w http.ResponseWriter, r *http.Request) {
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

	var req addDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if !hostnameRegex.MatchString(req.Hostname) {
		writeError(w, http.StatusBadRequest, "invalid hostname format")
		return
	}

	// Defaults
	sslMode := "automatic"
	if req.SSLMode != "" {
		if !validSSLModes[req.SSLMode] {
			writeError(w, http.StatusBadRequest, "invalid ssl_mode: must be automatic, dns_challenge, custom, or off")
			return
		}
		sslMode = req.SSLMode
	}
	forceHTTPS := req.SSLEnabled
	if req.ForceHTTPS != nil {
		forceHTTPS = *req.ForceHTTPS
	}

	params := generated.CreateDomainParams{
		ApplicationID:  applicationUUID,
		Hostname:       req.Hostname,
		SslEnabled:     req.SSLEnabled,
		ForceHttps:     forceHTTPS,
		SslMode:        sslMode,
		SslProvider:    pgtype.Text{String: req.SSLProvider, Valid: req.SSLProvider != ""},
		CertPath:       pgtype.Text{String: req.CertPath, Valid: req.CertPath != ""},
		KeyPath:        pgtype.Text{String: req.KeyPath, Valid: req.KeyPath != ""},
		AdvancedConfig: req.AdvancedConfig,
	}
	if req.ContainerPort != nil {
		params.ContainerPort = pgtype.Int4{Int32: *req.ContainerPort, Valid: true}
	}

	domain, err := h.queries.CreateDomain(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add domain")
		return
	}

	// Add Caddy proxy route for this domain
	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve project")
		return
	}

	var port int32 = 8080
	if req.ContainerPort != nil {
		port = *req.ContainerPort
	}

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.proxy.AddRoute(r.Context(), proxy.RouteConfig{
		Hostname:       req.Hostname,
		TargetURL:      fmt.Sprintf("http://%s:%d", containerName, port),
		TLS:            req.SSLEnabled,
		ForceHTTPS:     forceHTTPS,
		SSLMode:        sslMode,
		CertPath:       req.CertPath,
		KeyPath:        req.KeyPath,
		AdvancedConfig: req.AdvancedConfig,
	}); err != nil {
		slog.Error("failed to add proxy route for domain", "hostname", req.Hostname, "container", containerName, "error", err)
		if delErr := h.queries.DeleteDomain(r.Context(), domain.ID); delErr != nil {
			slog.Error("failed to rollback domain insert after proxy failure", "domain_id", domain.ID, "error", delErr)
		}
		writeError(w, http.StatusInternalServerError, "failed to configure proxy for domain")
		return
	}

	h.audit(r, "add_domain", "domain", uuidToString(domain.ID), map[string]any{"hostname": req.Hostname})

	writeJSON(w, http.StatusCreated, domain)
}

type updateDomainRequest struct {
	Hostname       string          `json:"hostname"`
	SSLEnabled     bool            `json:"ssl_enabled"`
	ContainerPort  *int32          `json:"container_port,omitempty"`
	ForceHTTPS     bool            `json:"force_https"`
	SSLMode        string          `json:"ssl_mode"`
	SSLProvider    string          `json:"ssl_provider,omitempty"`
	CertPath       string          `json:"cert_path,omitempty"`
	KeyPath        string          `json:"key_path,omitempty"`
	AdvancedConfig json.RawMessage `json:"advanced_config,omitempty"`
}

func (h *Handler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	if !h.canAccessDomain(r, domainUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req updateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Hostname == "" {
		writeError(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if !hostnameRegex.MatchString(req.Hostname) {
		writeError(w, http.StatusBadRequest, "invalid hostname format")
		return
	}
	if req.SSLMode != "" && !validSSLModes[req.SSLMode] {
		writeError(w, http.StatusBadRequest, "invalid ssl_mode")
		return
	}
	if req.SSLMode == "" {
		req.SSLMode = "automatic"
	}

	// Fetch existing domain to detect hostname changes
	oldDomain, err := h.queries.GetDomain(r.Context(), domainUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	params := generated.UpdateDomainParams{
		ID:             domainUUID,
		Hostname:       req.Hostname,
		SslEnabled:     req.SSLEnabled,
		ForceHttps:     req.ForceHTTPS,
		SslMode:        req.SSLMode,
		SslProvider:    pgtype.Text{String: req.SSLProvider, Valid: req.SSLProvider != ""},
		CertPath:       pgtype.Text{String: req.CertPath, Valid: req.CertPath != ""},
		KeyPath:        pgtype.Text{String: req.KeyPath, Valid: req.KeyPath != ""},
		AdvancedConfig: req.AdvancedConfig,
	}
	if req.ContainerPort != nil {
		params.ContainerPort = pgtype.Int4{Int32: *req.ContainerPort, Valid: true}
	}

	domain, err := h.queries.UpdateDomain(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update domain")
		return
	}

	// Rebuild proxy route: remove old hostname (which may equal new), then
	// AddRoute below installs the fresh config. Failure here is logged but
	// non-fatal — a stale route is preferable to a 500 on the update endpoint.
	if err := h.proxy.RemoveRoute(r.Context(), oldDomain.Hostname); err != nil {
		slog.Warn("update domain: failed to remove stale proxy route", "hostname", oldDomain.Hostname, "error", err)
	}

	// Resolve container name and port for the proxy route
	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), domain.ApplicationID)
	if err != nil {
		slog.Error("failed to resolve application for proxy update", "error", err)
		writeJSON(w, http.StatusOK, domain)
		return
	}

	var port int32 = 8080
	if req.ContainerPort != nil {
		port = *req.ContainerPort
	}

	// Load route features for this domain
	features := h.loadRouteFeatures(r, domainUUID)

	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, uuidToString(domain.ApplicationID))
	if err := h.proxy.AddRoute(r.Context(), proxy.RouteConfig{
		Hostname:       req.Hostname,
		TargetURL:      fmt.Sprintf("http://%s:%d", containerName, port),
		TLS:            req.SSLEnabled,
		ForceHTTPS:     req.ForceHTTPS,
		SSLMode:        req.SSLMode,
		SSLProvider:    req.SSLProvider,
		CertPath:       req.CertPath,
		KeyPath:        req.KeyPath,
		Features:       features,
		AdvancedConfig: req.AdvancedConfig,
	}); err != nil {
		slog.Error("failed to update proxy route", "hostname", req.Hostname, "error", err)
	}

	h.audit(r, "update_domain", "domain", domainID, map[string]any{"hostname": req.Hostname})

	writeJSON(w, http.StatusOK, domain)
}

// loadRouteFeatures fetches route features from DB and converts to proxy.RouteFeature slice.
func (h *Handler) loadRouteFeatures(r *http.Request, domainID pgtype.UUID) []proxy.RouteFeature {
	dbFeatures, err := h.queries.ListRouteFeaturesByDomain(r.Context(), domainID)
	if err != nil {
		slog.Warn("failed to load route features", "domain_id", domainID, "error", err)
		return nil
	}
	var features []proxy.RouteFeature
	for _, f := range dbFeatures {
		features = append(features, proxy.RouteFeature{
			Type:    f.FeatureType,
			Config:  json.RawMessage(f.Config),
			Enabled: f.Enabled,
		})
	}
	return features
}

// --- Route Feature CRUD ---

func (h *Handler) ListRouteFeatures(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	if !h.canAccessDomain(r, domainUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	features, err := h.queries.ListRouteFeaturesByDomain(r.Context(), domainUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list route features")
		return
	}

	writeJSON(w, http.StatusOK, features)
}

type upsertRouteFeatureRequest struct {
	FeatureType string          `json:"feature_type"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
}

func (h *Handler) UpsertRouteFeature(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	if !h.canAccessDomain(r, domainUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	var req upsertRouteFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validFeatureTypes[req.FeatureType] {
		writeError(w, http.StatusBadRequest, "invalid feature_type: must be basic_auth, redirect, headers, ip_allowlist, or rate_limit")
		return
	}

	if _, err := proxy.ParseFeatureConfig(req.FeatureType, req.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	feature, err := h.queries.UpsertRouteFeature(r.Context(), generated.UpsertRouteFeatureParams{
		DomainID:    domainUUID,
		FeatureType: req.FeatureType,
		Config:      req.Config,
		Enabled:     req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert route feature")
		return
	}

	// Rebuild proxy route with updated features
	h.rebuildDomainRoute(r, domainUUID)

	writeJSON(w, http.StatusOK, feature)
}

func (h *Handler) DeleteRouteFeature(w http.ResponseWriter, r *http.Request) {
	featureID := chi.URLParam(r, "featureId")
	var featureUUID pgtype.UUID
	if err := featureUUID.Scan(featureID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid feature id")
		return
	}

	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	if !h.canAccessDomain(r, domainUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.queries.DeleteRouteFeature(r.Context(), featureUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete route feature")
		return
	}

	// Rebuild proxy route with updated features
	h.rebuildDomainRoute(r, domainUUID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// rebuildDomainRoute removes and re-adds the proxy route for a domain with current config + features.
func (h *Handler) rebuildDomainRoute(r *http.Request, domainID pgtype.UUID) {
	domain, err := h.queries.GetDomain(r.Context(), domainID)
	if err != nil {
		slog.Warn("rebuildDomainRoute: failed to get domain", "error", err)
		return
	}

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), domain.ApplicationID)
	if err != nil {
		slog.Warn("rebuildDomainRoute: failed to resolve application", "error", err)
		return
	}

	var port int32 = 8080
	if domain.ContainerPort.Valid {
		port = domain.ContainerPort.Int32
	}

	features := h.loadRouteFeatures(r, domainID)

	appIDStr := uuidToString(domain.ApplicationID)
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, appIDStr)

	if err := h.proxy.RemoveRoute(r.Context(), domain.Hostname); err != nil {
		slog.Warn("rebuildDomainRoute: failed to remove proxy route", "hostname", domain.Hostname, "error", err)
	}
	if err := h.proxy.AddRoute(r.Context(), proxy.RouteConfig{
		Hostname:       domain.Hostname,
		TargetURL:      fmt.Sprintf("http://%s:%d", containerName, port),
		TLS:            domain.SslEnabled,
		ForceHTTPS:     domain.ForceHttps,
		SSLMode:        domain.SslMode,
		SSLProvider:    domain.SslProvider.String,
		CertPath:       domain.CertPath.String,
		KeyPath:        domain.KeyPath.String,
		Features:       features,
		AdvancedConfig: domain.AdvancedConfig,
	}); err != nil {
		slog.Warn("rebuildDomainRoute: failed to add proxy route", "hostname", domain.Hostname, "error", err)
	}
}

func (h *Handler) RemoveDomain(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	if !h.canAccessDomain(r, domainUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Fetch domain to get hostname for proxy removal
	domain, err := h.queries.GetDomain(r.Context(), domainUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// Remove Caddy proxy route first — if it fails, don't delete from DB
	if err := h.proxy.RemoveRoute(r.Context(), domain.Hostname); err != nil {
		slog.Error("failed to remove proxy route for domain", "hostname", domain.Hostname, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove proxy route for domain")
		return
	}

	// Delete from database
	if err := h.queries.DeleteDomain(r.Context(), domainUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove domain")
		return
	}

	h.audit(r, "remove_domain", "domain", domainID, map[string]any{"hostname": domain.Hostname})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
