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
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Scopes      []string           `json:"scopes"`
	RoleAtIssue string             `json:"role_at_issue"`
	ExpiresAt   pgtype.Timestamptz `json:"expires_at"`
	LastUsedAt  pgtype.Timestamptz `json:"last_used_at"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

func tokenDTOFromRow(row generated.ListAPITokensByUserRow) apiTokenDTO {
	return apiTokenDTO{
		ID:          uuidToString(row.ID),
		Name:        row.Name,
		Scopes:      row.Scopes,
		RoleAtIssue: row.RoleAtIssue,
		ExpiresAt:   row.ExpiresAt,
		LastUsedAt:  row.LastUsedAt,
		CreatedAt:   row.CreatedAt,
	}
}

// ListAPITokens returns the current user's own tokens — never another user's.
// There is no cross-user token oversight view in v1, admin or not.
// GET /api/tokens
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
// in a value PR4's enforcement doesn't know about. Unpinned to any project
// (every project the owner can reach, evaluated at use time); narrowing by
// project has no UI yet.
// POST /api/tokens
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

	created, err := h.tokenSvc.Create(r.Context(), service.CreateTokenParams{
		UserID:      userUUID,
		Name:        req.Name,
		RoleAtIssue: middleware.RoleFromContext(r.Context()),
		ExpiresAt:   expiresAt,
		Scopes:      scopes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	h.audit(r, "token_created", "api_token", uuidToString(created.ID), map[string]any{
		"name": created.Name,
	})

	// The only response that ever carries the plaintext — shown once, never
	// stored or logged past this point.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            uuidToString(created.ID),
		"name":          created.Name,
		"token":         created.Plain,
		"scopes":        created.Scopes,
		"role_at_issue": created.RoleAtIssue,
		"expires_at":    created.ExpiresAt,
		"created_at":    created.CreatedAt,
	})
}

// DeleteAPIToken revokes one of the current user's own tokens. The query
// itself is scoped by user_id (not just an authz check beforehand), so it can
// never delete another user's token even given that token's id.
// DELETE /api/tokens/{tokenId}
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
