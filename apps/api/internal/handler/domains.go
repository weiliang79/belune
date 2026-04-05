package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/proxy"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

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

	domains, err := h.queries.ListDomainsByApplication(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list domains")
		return
	}

	writeJSON(w, http.StatusOK, domains)
}

type addDomainRequest struct {
	Hostname   string `json:"hostname"`
	SSLEnabled bool   `json:"ssl_enabled"`
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

	// Insert domain record
	domain, err := h.queries.CreateDomain(r.Context(), generated.CreateDomainParams{
		ApplicationID: applicationUUID,
		Hostname:      req.Hostname,
		SslEnabled:    req.SSLEnabled,
		ForceHttps:    req.SSLEnabled,
		SslMode:       "automatic",
	})
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
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)
	if err := h.proxy.AddRoute(r.Context(), proxy.RouteConfig{
		Hostname:  req.Hostname,
		TargetURL: fmt.Sprintf("http://%s:%d", containerName, row.Port),
		TLS:       req.SSLEnabled,
	}); err != nil {
		slog.Error("failed to add proxy route for domain", "hostname", req.Hostname, "container", containerName, "error", err)
		// Rollback the domain DB insert since proxy registration failed
		if delErr := h.queries.DeleteDomain(r.Context(), domain.ID); delErr != nil {
			slog.Error("failed to rollback domain insert after proxy failure", "domain_id", domain.ID, "error", delErr)
		}
		writeError(w, http.StatusInternalServerError, "failed to configure proxy for domain")
		return
	}

	writeJSON(w, http.StatusCreated, domain)
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
