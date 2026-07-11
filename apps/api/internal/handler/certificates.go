package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/server/middleware"
	"github.com/weiling79/belune/internal/service"
)

// ListCertificates returns metadata for every stored certificate. Responses
// carry no key material — only what the leaf declares publicly.
func (h *Handler) ListCertificates(w http.ResponseWriter, r *http.Request) {
	certs, err := h.certSvc.ListCertificates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	writeJSON(w, http.StatusOK, certs)
}

type uploadCertificateRequest struct {
	Name    string `json:"name"`
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

// UploadCertificate stores a PEM pair after validating it. Validation errors are
// the operator's to fix, so they come back as 400 with the specific reason
// (mismatched key, missing SANs, not PEM at all) rather than a generic failure.
func (h *Handler) UploadCertificate(w http.ResponseWriter, r *http.Request) {
	var req uploadCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var createdBy pgtype.UUID
	if err := createdBy.Scan(middleware.UserIDFromContext(r.Context())); err != nil {
		createdBy = pgtype.UUID{}
	}

	cert, err := h.certSvc.CreateCertificate(r.Context(), req.Name, req.CertPEM, req.KeyPEM, createdBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "a certificate with that name already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.audit(r, "upload_certificate", "certificate", cert.ID, map[string]any{
		"name":     cert.Name,
		"subjects": cert.Subjects,
	})

	writeJSON(w, http.StatusCreated, cert)
}

// DeleteCertificate removes a certificate unless domains still serve it.
func (h *Handler) DeleteCertificate(w http.ResponseWriter, r *http.Request) {
	certificateID := chi.URLParam(r, "certificateId")
	var certUUID pgtype.UUID
	if err := certUUID.Scan(certificateID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid certificate id")
		return
	}

	if err := h.certSvc.DeleteCertificate(r.Context(), certUUID); err != nil {
		if errors.Is(err, service.ErrCertificateInUse) {
			// The message names the domains still referencing it, so the operator
			// knows exactly what to detach first.
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete certificate")
		return
	}

	h.audit(r, "delete_certificate", "certificate", certificateID, nil)

	w.WriteHeader(http.StatusNoContent)
}
