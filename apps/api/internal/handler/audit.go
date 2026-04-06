package handler

import (
	"net/http"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// ListAuditLogs returns paginated audit logs (admin-only).
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	logs, err := h.queries.ListAuditLogs(r.Context(), generated.ListAuditLogsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	count, err := h.queries.CountAuditLogs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count audit logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": logs,
		"total": count,
	})
}
