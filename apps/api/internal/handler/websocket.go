package handler

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"nhooyr.io/websocket"

	"github.com/weiliang79/belune/internal/server/middleware"
	"github.com/weiliang79/belune/internal/ws"
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

	// Validate the Origin header to prevent cross-site WebSocket hijacking (CSWSH).
	if origin := r.Header.Get("Origin"); origin != "" && !h.originAllowed(r, origin) {
		slog.Warn("ws: rejected connection from disallowed origin", "origin", origin, "host", r.Host)
		writeError(w, http.StatusForbidden, "origin not allowed")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Origin is validated by originAllowed above; skip the library's own
		// host-match check so a configured cross-origin client (the Vite dev
		// server on another port) can still connect.
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

// originAllowed reports whether a browser Origin may open a WebSocket.
//
// Same-origin is always allowed: the page making the request was served by this
// very host, so it cannot be a cross-site hijack. This has to be derived from the
// request rather than configured, because the dashboard's hostname is set at
// runtime from the UI — a static allowlist would reject the panel's own socket
// the moment an operator pointed a domain at it, and again on every change.
//
// CORS_ORIGINS remains for genuinely cross-origin clients, i.e. the Vite dev
// server on :5173 talking to the API on :8080.
func (h *Handler) originAllowed(r *http.Request, origin string) bool {
	if u, err := url.Parse(origin); err == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, o := range h.cfg.CORSOrigins {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}
