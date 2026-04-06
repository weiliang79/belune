package handler

import (
	"log/slog"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
)

// HandleWebSocket upgrades the connection to WebSocket and registers the client.
// Authentication is performed via the session cookie (WebSocket can't send custom headers).
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if h.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "websocket not available")
		return
	}

	// Extract user ID from auth context (set by auth middleware)
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow all origins in dev; in production, configure via CORS
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("ws: failed to accept connection", "error", err)
		return
	}

	client := ws.NewClient(h.hub, conn, userID)
	if !h.hub.Register(client) {
		conn.Close(websocket.StatusTryAgainLater, "too many connections")
		return
	}

	// Start read and write pumps
	go client.WritePump(r.Context())
	client.ReadPump(r.Context())
}
