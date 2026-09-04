package handler

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/server/middleware"
)

// canAccessOwned reports whether the current request's user may access a resource
// given its owner and whether its parent project is shared. Admins always pass;
// a shared project grants every Member the same access as its owner.
func (h *Handler) canAccessOwned(r *http.Request, getOwner func(ctx context.Context) (ownerID pgtype.UUID, shared bool, err error)) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, shared, err := getOwner(r.Context())
	if err != nil {
		return false
	}
	if shared {
		return true
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}

// canAccessApplication checks if the current user owns, or shares access to,
// the application's parent project. Admins bypass the check.
func (h *Handler) canAccessApplication(r *http.Request, applicationID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetApplicationOwnerUserID(ctx, applicationID)
		return row.UserID, row.Shared, err
	})
}

// canAccessDatabase checks if the current user owns, or shares access to, the
// database's parent project. Admins bypass the check.
func (h *Handler) canAccessDatabase(r *http.Request, databaseID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetDatabaseOwnerUserID(ctx, databaseID)
		return row.UserID, row.Shared, err
	})
}

// canAccessIntegration checks if the current user created the git connection.
// Admins bypass the check. Git integrations are user-level resources with no
// project relationship, so project sharing never applies here.
func (h *Handler) canAccessIntegration(r *http.Request, integrationID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, bool, error) {
		ownerID, err := h.gitIntegrationSvc.OwnerID(ctx, integrationID)
		return ownerID, false, err
	})
}

// canAccessDomain checks if the current user owns, or shares access to, the
// domain's parent application's parent project. Admins bypass the check.
func (h *Handler) canAccessDomain(r *http.Request, domainID pgtype.UUID) bool {
	return h.canAccessOwned(r, func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetDomainOwnerUserID(ctx, domainID)
		return row.UserID, row.Shared, err
	})
}
