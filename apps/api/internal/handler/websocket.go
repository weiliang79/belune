package handler

import (
	"log/slog"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/weiling79/belune/internal/server/middleware"
	"github.com/weiling79/belune/internal/ws"
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

	// Validate the Origin header against the configured allowed origins to prevent
	// cross-site WebSocket hijacking (CSWSH). Browsers always send Origin; requests
	// without an Origin header (same-origin or non-browser clients) are allowed through.
	if origin := r.Header.Get("Origin"); origin != "" {
		allowed := false
		for _, o := range h.cfg.CORSOrigins {
			if o == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			slog.Warn("ws: rejected connection from disallowed origin", "origin", origin)
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Origin is validated manually above; skip the library's host-match check
		// so that allowed cross-origin clients (e.g. frontend on a different port) can connect.
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
