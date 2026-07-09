package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/pkg/sse"
	"github.com/weiling79/belune/internal/store/generated"
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

// requestLogFilter holds the shared filter columns used by the list, summary,
// and per-minute request-log queries.
type requestLogFilter struct {
	ApplicationID pgtype.UUID
	StatusMin     pgtype.Int2
	StatusMax     pgtype.Int2
	From          pgtype.Timestamptz
	To            pgtype.Timestamptz
	Search        pgtype.Text
}

// parseRequestLogFilter reads the request-log filter query params. Invalid
// values are ignored (left as SQL NULL) rather than erroring, matching the
// existing best-effort parsing.
func parseRequestLogFilter(r *http.Request) requestLogFilter {
	q := r.URL.Query()
	var f requestLogFilter
	if v := q.Get("application_id"); v != "" {
		var id pgtype.UUID
		if err := id.Scan(v); err == nil {
			f.ApplicationID = id
		}
	}
	if v := q.Get("status_min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.StatusMin = pgtype.Int2{Int16: int16(n), Valid: true}
		}
	}
	if v := q.Get("status_max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.StatusMax = pgtype.Int2{Int16: int16(n), Valid: true}
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := q.Get("search"); v != "" {
		f.Search = pgtype.Text{String: v, Valid: true}
	}
	return f
}

// ListAllRequestLogs returns paginated, filterable HTTP request logs across all applications (admin only).
// GET /api/requests?application_id=&status_min=&status_max=&from=&to=&search=
func (h *Handler) ListAllRequestLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	f := parseRequestLogFilter(r)

	logs, err := h.queries.ListRequestLogsFiltered(r.Context(), generated.ListRequestLogsFilteredParams{
		Limit:         limit,
		Offset:        offset,
		ApplicationID: f.ApplicationID,
		StatusMin:     f.StatusMin,
		StatusMax:     f.StatusMax,
		From:          f.From,
		To:            f.To,
		Search:        f.Search,
	})
	if err != nil {
		slog.Error("failed to list all request logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list request logs")
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

type requestClassCounts struct {
	C2xx int64 `json:"c2xx"`
	C3xx int64 `json:"c3xx"`
	C4xx int64 `json:"c4xx"`
	C5xx int64 `json:"c5xx"`
}

type requestPerMinutePoint struct {
	Ts    string `json:"ts"`
	Count int64  `json:"count"`
}

type requestSummaryResponse struct {
	Total       int64                   `json:"total"`
	ClassCounts requestClassCounts      `json:"class_counts"`
	P50Ms       float64                 `json:"p50_ms"`
	P95Ms       float64                 `json:"p95_ms"`
	ErrorRate   float64                 `json:"error_rate"`
	PerMinute   []requestPerMinutePoint `json:"per_minute"`
}

// GetAllRequestsSummary returns status-class counts, latency percentiles, error
// rate, and a per-minute series for the requests dashboard (admin only). It
// honors the same filters as ListAllRequestLogs.
// GET /api/requests/summary
func (h *Handler) GetAllRequestsSummary(w http.ResponseWriter, r *http.Request) {
	f := parseRequestLogFilter(r)
	ctx := r.Context()

	sum, err := h.queries.GetRequestSummary(ctx, generated.GetRequestSummaryParams{
		ApplicationID: f.ApplicationID,
		StatusMin:     f.StatusMin,
		StatusMax:     f.StatusMax,
		From:          f.From,
		To:            f.To,
		Search:        f.Search,
	})
	if err != nil {
		slog.Error("failed to summarize request logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to summarize request logs")
		return
	}

	buckets, err := h.queries.GetRequestPerMinute(ctx, generated.GetRequestPerMinuteParams{
		ApplicationID: f.ApplicationID,
		StatusMin:     f.StatusMin,
		StatusMax:     f.StatusMax,
		From:          f.From,
		To:            f.To,
		Search:        f.Search,
	})
	if err != nil {
		slog.Error("failed to bucket request logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to summarize request logs")
		return
	}

	// The query returns most-recent first; reverse to chronological for charting.
	perMinute := make([]requestPerMinutePoint, 0, len(buckets))
	for i := len(buckets) - 1; i >= 0; i-- {
		perMinute = append(perMinute, requestPerMinutePoint{
			Ts:    buckets[i].Bucket.Time.UTC().Format(time.RFC3339),
			Count: buckets[i].Count,
		})
	}

	var errorRate float64
	if sum.Total > 0 {
		errorRate = float64(sum.Class5xx) / float64(sum.Total) * 100
	}

	writeJSON(w, http.StatusOK, requestSummaryResponse{
		Total: sum.Total,
		ClassCounts: requestClassCounts{
			C2xx: sum.Class2xx,
			C3xx: sum.Class3xx,
			C4xx: sum.Class4xx,
			C5xx: sum.Class5xx,
		},
		P50Ms:     sum.P50Ms,
		P95Ms:     sum.P95Ms,
		ErrorRate: errorRate,
		PerMinute: perMinute,
	})
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
