package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// ListRequestLogs returns paginated HTTP request logs for an application.
// GET /api/projects/{projectId}/applications/{applicationId}/requests
func (h *Handler) ListRequestLogs(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	logs, err := h.queries.ListRequestLogsByApplication(r.Context(), generated.ListRequestLogsByApplicationParams{
		ApplicationID: appUUID,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list request logs")
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// StreamRequestLogs streams live HTTP request logs for an application via SSE.
// GET /api/projects/{projectId}/applications/{applicationId}/requests/stream
func (h *Handler) StreamRequestLogs(w http.ResponseWriter, r *http.Request) {
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

	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()
	if err := writer.SendComment("ping"); err != nil {
		return
	}

	channel := "requests:live:" + applicationID
	pubsub := h.rdb.Subscribe(ctx, channel)
	defer pubsub.Close()
	ch := pubsub.Channel()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := writer.SendComment("ping"); err != nil {
				return
			}
		case msg := <-ch:
			if err := writer.SendData(msg.Payload); err != nil {
				return
			}
		}
	}
}

// ListAllRequestLogs returns paginated, filterable HTTP request logs across all applications (admin only).
// GET /api/requests?application_id=&status_min=&status_max=&from=&to=
func (h *Handler) ListAllRequestLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	q := r.URL.Query()

	params := generated.ListRequestLogsFilteredParams{Limit: limit, Offset: offset}

	if v := q.Get("application_id"); v != "" {
		var id pgtype.UUID
		if err := id.Scan(v); err == nil {
			params.ApplicationID = id
		}
	}
	if v := q.Get("status_min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.StatusMin = pgtype.Int2{Int16: int16(n), Valid: true}
		}
	}
	if v := q.Get("status_max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.StatusMax = pgtype.Int2{Int16: int16(n), Valid: true}
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.From = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.To = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	logs, err := h.queries.ListRequestLogsFiltered(r.Context(), params)
	if err != nil {
		slog.Error("failed to list all request logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list request logs")
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// StreamAllRequestLogs streams live request logs across all apps via SSE (admin only).
// GET /api/requests/stream
func (h *Handler) StreamAllRequestLogs(w http.ResponseWriter, r *http.Request) {
	writer, err := sse.NewWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()
	if err := writer.SendComment("ping"); err != nil {
		return
	}

	// Subscribe to all app request log channels using pattern
	pubsub := h.rdb.PSubscribe(ctx, "requests:live:*")
	defer pubsub.Close()
	ch := pubsub.Channel()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := writer.SendComment("ping"); err != nil {
				return
			}
		case msg := <-ch:
			// Enrich with application_id extracted from channel name
			var payload map[string]any
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err == nil {
				// Channel is "requests:live:{applicationId}"
				parts := msg.Channel
				if len(parts) > len("requests:live:") {
					payload["application_id"] = parts[len("requests:live:"):]
				}
				enriched, _ := json.Marshal(payload)
				if err := writer.SendData(string(enriched)); err != nil {
					return
				}
			}
		}
	}
}

// parsePagination reads limit/offset query params, defaulting to 50/0.
func parsePagination(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}
