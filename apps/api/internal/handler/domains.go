package handler

import (
	"encoding/json"
	"fmt"
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
		Hostname:   req.Hostname,
		SslEnabled: req.SSLEnabled,
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
	_ = h.proxy.AddRoute(r.Context(), proxy.RouteConfig{
		Hostname:  req.Hostname,
		TargetURL: fmt.Sprintf("http://%s:8080", containerName),
		TLS:       req.SSLEnabled,
	})

	writeJSON(w, http.StatusCreated, domain)
}

func (h *Handler) RemoveDomain(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	var domainUUID pgtype.UUID
	if err := domainUUID.Scan(domainID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	// Fetch domain to get hostname for proxy removal
	domain, err := h.queries.GetDomain(r.Context(), domainUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// Remove Caddy proxy route
	_ = h.proxy.RemoveRoute(r.Context(), domain.Hostname)

	// Delete from database
	if err := h.queries.DeleteDomain(r.Context(), domainUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove domain")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
