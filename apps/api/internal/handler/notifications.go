package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/pkg/sse"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/service"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// currentUserUUID resolves the authenticated user's id as a pgtype.UUID.
func currentUserUUID(r *http.Request) (pgtype.UUID, bool) {
	var uuid pgtype.UUID
	if err := uuid.Scan(middleware.UserIDFromContext(r.Context())); err != nil {
		return uuid, false
	}
	return uuid, true
}

// ListNotifications returns the current user's notifications, newest first,
// along with the current unread count so the bell can render in one request.
// GET /api/notifications?limit=&offset=
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit, offset := parsePagination(r)

	items, err := h.queries.ListNotifications(r.Context(), generated.ListNotificationsParams{
		UserID: userUUID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	unread, err := h.queries.CountUnreadNotifications(r.Context(), userUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"unread": unread,
	})
}

// UnreadNotificationCount returns just the unread count — a cheap poll target
// and the value the SSE stream keeps live between full reloads.
// GET /api/notifications/unread-count
func (h *Handler) UnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	unread, err := h.queries.CountUnreadNotifications(r.Context(), userUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": unread})
}

// MarkNotificationRead marks a single notification read. Scoped by user_id so a
// user can only mark their own rows.
// POST /api/notifications/{notificationId}/read
func (h *Handler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var notifUUID pgtype.UUID
	if err := notifUUID.Scan(chi.URLParam(r, "notificationId")); err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	row, err := h.queries.MarkNotificationRead(r.Context(), generated.MarkNotificationReadParams{
		ID:     notifUUID,
		UserID: userUUID,
	})
	if err != nil {
		// No row updated means it doesn't exist or isn't the caller's.
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// MarkAllNotificationsRead marks every unread notification for the user read.
// POST /api/notifications/read-all
func (h *Handler) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := currentUserUUID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.queries.MarkAllNotificationsRead(r.Context(), userUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark notifications read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StreamNotifications pushes new notifications for the current user via SSE.
// Subscribes to the per-user Redis channel the NotificationService publishes to.
// GET /api/notifications/stream
func (h *Handler) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
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

	pubsub := h.rdb.Subscribe(ctx, service.NotificationChannel(userID))
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
