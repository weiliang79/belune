package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// ListAuditLogs returns paginated, optionally filtered audit logs (admin-only).
// Query params: limit, offset, user_id, action, resource_type, from (RFC3339), to (RFC3339).
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := generated.ListAuditLogsFilteredParams{
		Limit:  limit,
		Offset: offset,
	}
	countParams := generated.CountAuditLogsFilteredParams{}

	if v := r.URL.Query().Get("user_id"); v != "" {
		var uuid pgtype.UUID
		if err := uuid.Scan(v); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id format")
			return
		}
		params.UserID = uuid
		countParams.UserID = uuid
	}
	if v := r.URL.Query().Get("action"); v != "" {
		params.Action = pgtype.Text{String: v, Valid: true}
		countParams.Action = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("resource_type"); v != "" {
		params.ResourceType = pgtype.Text{String: v, Valid: true}
		countParams.ResourceType = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from format, expected RFC3339")
			return
		}
		params.From = pgtype.Timestamptz{Time: t, Valid: true}
		countParams.From = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to format, expected RFC3339")
			return
		}
		params.To = pgtype.Timestamptz{Time: t, Valid: true}
		countParams.To = pgtype.Timestamptz{Time: t, Valid: true}
	}

	logs, err := h.queries.ListAuditLogsFiltered(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	count, err := h.queries.CountAuditLogsFiltered(r.Context(), countParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count audit logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": logs,
		"total": count,
	})
}
