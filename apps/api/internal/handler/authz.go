package handler

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/server/middleware"
)

// canAccessOwned reports whether the current request's user may access a resource
// whose owner is returned by getOwnerID. Admins always pass.
func (h *Handler) canAccessOwned(r *http.Request, getOwnerID func(ctx context.Context) (pgtype.UUID, error)) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, err := getOwnerID(r.Context())
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}

// canAccessApplication checks if the current user owns the application's parent project.
// Admins bypass the check.
func (h *Handler) canAccessApplication(r *http.Request, applicationID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, error) {
		return h.queries.GetApplicationOwnerUserID(ctx, applicationID)
	})
}

// canAccessDatabase checks if the current user owns the database's parent project.
// Admins bypass the check.
func (h *Handler) canAccessDatabase(r *http.Request, databaseID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, error) {
		return h.queries.GetDatabaseOwnerUserID(ctx, databaseID)
	})
}

// canAccessIntegration checks if the current user created the git connection.
// Admins bypass the check.
func (h *Handler) canAccessIntegration(r *http.Request, integrationID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, error) {
		return h.gitIntegrationSvc.OwnerID(ctx, integrationID)
	})
}

// canAccessDomain checks if the current user owns the domain's parent application's parent project.
// Admins bypass the check.
func (h *Handler) canAccessDomain(r *http.Request, domainID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, error) {
		return h.queries.GetDomainOwnerUserID(ctx, domainID)
	})
}
