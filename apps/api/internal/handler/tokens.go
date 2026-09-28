package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/service"
	"github.com/weiliang79/belune/internal/store/generated"
)

// validTokenExpiryDays mirrors the UI's expiry picker (1/7/14/30/60/90 days).
// Enforced here too, so a client cannot mint an expiry the UI never offers.
// Absent entirely means "never expires".
var validTokenExpiryDays = map[int]bool{1: true, 7: true, 14: true, 30: true, 60: true, 90: true}

// apiTokenDTO is what a token looks like everywhere except the moment it is
// created — never the hash, never the plaintext.
type apiTokenDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	RoleAtIssue string   `json:"role_at_issue"`
	// ProjectID is the pin, marshaling to null when the token is unpinned
	// (pgtype.UUID's own MarshalJSON already does this) — every project the
	// owner can reach, evaluated at use time.
	ProjectID  pgtype.UUID        `json:"project_id"`
	ExpiresAt  pgtype.Timestamptz `json:"expires_at"`
	LastUsedAt pgtype.Timestamptz `json:"last_used_at"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
}

func tokenDTOFromRow(row generated.ListAPITokensByUserRow) apiTokenDTO {
	return apiTokenDTO{
		ID:          uuidToString(row.ID),
		Name:        row.Name,
		Scopes:      row.Scopes,
		RoleAtIssue: row.RoleAtIssue,
		ProjectID:   row.ProjectID,
		ExpiresAt:   row.ExpiresAt,
		LastUsedAt:  row.LastUsedAt,
		CreatedAt:   row.CreatedAt,
	}
}

// ListAPITokens returns the current user's own tokens — never another user's.
// There is no cross-user token oversight view in v1, admin or not.
// GET /api/tokens
//
//apidoc:tag tokens
func (h *Handler) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.queries.ListAPITokensByUser(r.Context(), userUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}

	dtos := make([]apiTokenDTO, len(rows))
	for i, row := range rows {
		dtos[i] = tokenDTOFromRow(row)
	}
	writeJSON(w, http.StatusOK, dtos)
}

type createTokenRequest struct {
	Name          string   `json:"name"`
	ExpiresInDays *int     `json:"expires_in_days"`
	Scopes        []string `json:"scopes"`
	// ProjectID is a pointer so an omitted field reads as "unpinned" (nil)
	// distinctly from an empty string, matching ExpiresInDays's own
	// omitted-vs-zero distinction. Validated below with canAccessProject —
	// a Member can pin to a project shared with them, not just one they own.
	ProjectID *string `json:"project_id"`
}

// validScopes indexes service.AllScopes for membership checks below.
var validScopes = func() map[string]bool {
	m := make(map[string]bool, len(service.AllScopes))
	for _, s := range service.AllScopes {
		m[s] = true
	}
	return m
}()

// normalizeScopes validates that every requested scope is a known one and
// returns the deduplicated set. An empty or all-invalid request is rejected
// outright — a token with zero scopes is a footgun (it authenticates but can
// do literally nothing), not a narrower valid choice.
func normalizeScopes(requested []string) ([]string, bool) {
	seen := make(map[string]bool, len(requested))
	out := make([]string, 0, len(requested))
	for _, s := range requested {
		if !validScopes[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// CreateAPIToken mints a token for the current user with exactly the scopes
// it requests — validated against service.AllScopes, so a client cannot smuggle
// in a value PR4's enforcement doesn't know about. Optionally pinned to one
// project the caller can already reach; omitted, it is unpinned (every
// project the owner can reach, evaluated at use time).
// POST /api/tokens
//
//apidoc:tag tokens
func (h *Handler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	scopes, ok := normalizeScopes(req.Scopes)
	if !ok {
		writeError(w, http.StatusBadRequest, "select at least one valid scope")
		return
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresInDays != nil {
		if !validTokenExpiryDays[*req.ExpiresInDays] {
			writeError(w, http.StatusBadRequest, "invalid expires_in_days")
			return
		}
		expiresAt = pgtype.Timestamptz{
			Time:  time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour),
			Valid: true,
		}
	}

	// canAccessProject, not isProjectOwner: a Member must be able to pin to a
	// project shared with them, or the feature is useless to exactly the
	// people who need it. This check is UX, not the security boundary — a
	// token's actual reach is owner's role ∩ project access ∩ scope,
	// evaluated at use time, so pinning can never grant access the owner
	// doesn't independently have. project_id is ON DELETE CASCADE: deleting
	// the pinned project silently deletes this token too.
	var pinnedProjectID pgtype.UUID
	if req.ProjectID != nil {
		if err := pinnedProjectID.Scan(*req.ProjectID); err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}
		if !h.canAccessProject(r, pinnedProjectID) {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	created, err := h.tokenSvc.Create(r.Context(), service.CreateTokenParams{
		UserID:      userUUID,
		Name:        req.Name,
		RoleAtIssue: middleware.RoleFromContext(r.Context()),
		ExpiresAt:   expiresAt,
		Scopes:      scopes,
		ProjectID:   pinnedProjectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	auditDetails := map[string]any{
		"name": created.Name,
	}
	if created.ProjectID.Valid {
		auditDetails["project_id"] = uuidToString(created.ProjectID)
	}
	h.audit(r, "token_created", "api_token", uuidToString(created.ID), auditDetails)

	// The only response that ever carries the plaintext — shown once, never
	// stored or logged past this point.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            uuidToString(created.ID),
		"name":          created.Name,
		"token":         created.Plain,
		"scopes":        created.Scopes,
		"role_at_issue": created.RoleAtIssue,
		"project_id":    created.ProjectID,
		"expires_at":    created.ExpiresAt,
		"created_at":    created.CreatedAt,
	})
}

// DeleteAPIToken revokes one of the current user's own tokens. The query
// itself is scoped by user_id (not just an authz check beforehand), so it can
// never delete another user's token even given that token's id.
// DELETE /api/tokens/{tokenId}
//
//apidoc:tag tokens
func (h *Handler) DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var tokenUUID pgtype.UUID
	if err := tokenUUID.Scan(chi.URLParam(r, "tokenId")); err != nil {
		writeError(w, http.StatusBadRequest, "invalid token id")
		return
	}

	name, err := h.queries.DeleteAPIToken(r.Context(), generated.DeleteAPITokenParams{
		ID:     tokenUUID,
		UserID: userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete token")
		return
	}

	h.audit(r, "token_deleted", "api_token", uuidToString(tokenUUID), map[string]any{
		"name": name,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
