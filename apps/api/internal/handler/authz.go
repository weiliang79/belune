package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
)

// canAccessApplication checks if the current user owns the application's parent project.
// Admins bypass the check.
func (h *Handler) canAccessApplication(r *http.Request, applicationID pgtype.UUID) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, err := h.queries.GetApplicationOwnerUserID(r.Context(), applicationID)
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}

// canAccessDatabase checks if the current user owns the database's parent project.
// Admins bypass the check.
func (h *Handler) canAccessDatabase(r *http.Request, databaseID pgtype.UUID) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, err := h.queries.GetDatabaseOwnerUserID(r.Context(), databaseID)
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}

// canAccessDomain checks if the current user owns the domain's parent application's parent project.
// Admins bypass the check.
func (h *Handler) canAccessDomain(r *http.Request, domainID pgtype.UUID) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, err := h.queries.GetDomainOwnerUserID(r.Context(), domainID)
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}
