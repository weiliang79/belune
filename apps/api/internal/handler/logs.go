package handler

import (
	"bufio"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var applicationUUID pgtype.UUID
	if err := applicationUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, applicationUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)

	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	follow := r.URL.Query().Get("follow") == "true"

	logs, err := h.runtime.ContainerLogs(r.Context(), containerName, follow)
	if err != nil {
		writer.SendEvent("error", fmt.Sprintf("failed to get logs: %v", err))
		return
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		if err := writer.SendData(line); err != nil {
			return // Client disconnected
		}
	}

	writer.SendEvent("done", "log stream ended")
}

// ListApplicationLogs returns paginated, filterable historical application logs.
// GET /api/projects/{projectId}/applications/{applicationId}/logs/history
//
// Query params:
//
//	q      — keyword search (case-insensitive substring match on message)
//	stream — filter by stream: "stdout" or "stderr"
//	since  — RFC3339 timestamp lower bound (inclusive)
//	until  — RFC3339 timestamp upper bound (inclusive)
//	limit  — page size (default 500, max 1000)
//	offset — pagination offset
func (h *Handler) ListApplicationLogs(w http.ResponseWriter, r *http.Request) {
	applicationID := chi.URLParam(r, "applicationId")
	var appUUID pgtype.UUID
	if err := appUUID.Scan(applicationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	if !h.canAccessApplication(r, appUUID) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	q := r.URL.Query()
	limit, offset := parseLogspagination(r)

	params := generated.SearchApplicationLogsParams{
		ApplicationID: appUUID,
		Limit:         limit,
		Offset:        offset,
	}
	if v := q.Get("q"); v != "" {
		params.Q = pgtype.Text{String: v, Valid: true}
	}
	if v := q.Get("stream"); v == "stdout" || v == "stderr" {
		params.Stream = pgtype.Text{String: v, Valid: true}
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.Since = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.Until = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	logs, err := h.queries.SearchApplicationLogs(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list application logs")
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// parseLogspagination is like parsePagination but with a higher default limit for log pages.
func parseLogspagination(r *http.Request) (int32, int32) {
	limit := int32(500)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		var parsed int32
		if n, err := fmt.Sscanf(v, "%d", &parsed); n == 1 && err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		var parsed int32
		if n, err := fmt.Sscanf(v, "%d", &parsed); n == 1 && err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}
