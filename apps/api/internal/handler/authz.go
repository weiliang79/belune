package handler

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/server/middleware"
)

// ownerLookup resolves a resource's owning user id and whether its parent
// project is shared.
type ownerLookup func(ctx context.Context) (ownerID pgtype.UUID, shared bool, err error)

// canAccessOwned reports whether the current request's user may access a
// resource given its owner and whether its parent project is shared. Admins
// always pass; a shared project grants every Member the same access as its
// owner. Use this for read/use access — destructive operations (delete,
// transfer, changing sharing) must use isOwnerOnly instead, since sharing
// grants operational access but never ownership.
func (h *Handler) canAccessOwned(r *http.Request, getOwner ownerLookup) bool {
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

// isOwnerOnly reports whether the current request's user owns the resource
// returned by getOwner. Admins always pass; unlike canAccessOwned, a shared
// project does NOT grant this — sharing widens operational access but never
// ownership, so this is what gates destroying the resource itself (as
// opposed to routine, recreatable config underneath it).
func (h *Handler) isOwnerOnly(r *http.Request, getOwner ownerLookup) bool {
	role := middleware.RoleFromContext(r.Context())
	if role == "admin" {
		return true
	}
	ownerID, _, err := getOwner(r.Context())
	if err != nil {
		return false
	}
	var userID pgtype.UUID
	userID.Scan(middleware.UserIDFromContext(r.Context()))
	return ownerID == userID
}

func (h *Handler) applicationOwner(applicationID pgtype.UUID) ownerLookup {
	return func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetApplicationOwnerUserID(ctx, applicationID)
		return row.UserID, row.Shared, err
	}
}

func (h *Handler) databaseOwner(databaseID pgtype.UUID) ownerLookup {
	return func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetDatabaseOwnerUserID(ctx, databaseID)
		return row.UserID, row.Shared, err
	}
}

func (h *Handler) domainOwner(domainID pgtype.UUID) ownerLookup {
	return func(ctx context.Context) (pgtype.UUID, bool, error) {
		row, err := h.queries.GetDomainOwnerUserID(ctx, domainID)
		return row.UserID, row.Shared, err
	}
}

// canAccessApplication checks if the current user owns, or shares access to,
// the application's parent project. Admins bypass the check.
func (h *Handler) canAccessApplication(r *http.Request, applicationID pgtype.UUID) bool {
	return h.canAccessOwned(r, h.applicationOwner(applicationID))
}

// isApplicationOwner checks if the current user owns the application's parent
// project. Admins bypass the check. Shared access does NOT pass — gates
// deleting the application itself and its volumes, not routine operation.
func (h *Handler) isApplicationOwner(r *http.Request, applicationID pgtype.UUID) bool {
	return h.isOwnerOnly(r, h.applicationOwner(applicationID))
}

// canAccessDatabase checks if the current user owns, or shares access to, the
// database's parent project. Admins bypass the check.
func (h *Handler) canAccessDatabase(r *http.Request, databaseID pgtype.UUID) bool {
	return h.canAccessOwned(r, h.databaseOwner(databaseID))
}

// isDatabaseOwner checks if the current user owns the database's parent
// project. Admins bypass the check. Shared access does NOT pass — gates
// deleting the database itself and its backups, not routine operation.
func (h *Handler) isDatabaseOwner(r *http.Request, databaseID pgtype.UUID) bool {
	return h.isOwnerOnly(r, h.databaseOwner(databaseID))
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
	return h.canAccessOwned(r, h.domainOwner(domainID))
}

// isDomainOwner checks if the current user owns the domain's parent
// application's parent project. Admins bypass the check. Shared access does
// NOT pass — gates removing the domain itself, not routine operation.
func (h *Handler) isDomainOwner(r *http.Request, domainID pgtype.UUID) bool {
	return h.isOwnerOnly(r, h.domainOwner(domainID))
}
