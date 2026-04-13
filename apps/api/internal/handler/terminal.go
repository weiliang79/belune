package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/server/middleware"
)

// termMsg is the JSON envelope for terminal WebSocket messages.
type termMsg struct {
	Type string `json:"type"`
	// output: base64-encoded terminal data
	Data string `json:"data,omitempty"`
	// resize
	Cols int `json:"cols,omitempty"`
	Rows int `json:"rows,omitempty"`
	// error / info
	Message string `json:"message,omitempty"`
}

// CreateTerminalSession creates a Docker exec session and returns a session ID.
// POST /api/projects/{projectId}/applications/{applicationId}/terminal
func (h *Handler) CreateTerminalSession(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Shell string `json:"shell"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Shell = ""
	}
	shell := req.Shell
	if shell == "" {
		shell = "sh"
	}
	// Restrict shell to known safe choices
	switch shell {
	case "sh", "bash", "ash", "zsh", "fish":
	default:
		shell = "sh"
	}

	row, err := h.queries.GetApplicationWithProjectSlug(r.Context(), applicationUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}
	containerName := naming.ContainerName(row.ProjectSlug, row.Slug, applicationID)

	sess, err := h.runtime.ContainerExecTTY(r.Context(), containerName, []string{shell})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("exec failed: %v", err))
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	s, ok := h.termManager.Create(applicationID, userID, shell, sess.ExecID, sess.RWC)
	if !ok {
		sess.RWC.Close()
		writeError(w, http.StatusTooManyRequests, "terminal session limit reached")
		return
	}

	h.audit(r, "terminal.session.started", "application", applicationID, map[string]any{
		"shell":      shell,
		"session_id": s.ID,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"session_id": s.ID})
}

// HandleTerminalWebSocket proxies a WebSocket connection to an active exec session.
// GET /api/ws/terminal/{sessionId}
func (h *Handler) HandleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if h.termManager == nil {
		writeError(w, http.StatusServiceUnavailable, "terminal not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	s := h.termManager.Get(sessionID)
	if s == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Auth: the WS route group already requires a valid session, but confirm
	// the requesting user owns this terminal session.
	userID := middleware.UserIDFromContext(r.Context())
	role := middleware.RoleFromContext(r.Context())
	if role != "admin" && s.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("terminal ws: accept failed", "error", err)
		return
	}
	defer func() {
		conn.Close(websocket.StatusNormalClosure, "")
		startedAt := s.CreatedAt
		h.termManager.Delete(sessionID)
		h.auditSvc.Log(s.UserID, r.RemoteAddr, "terminal.session.ended", "application", s.ApplicationID, map[string]any{
			"shell":      s.Shell,
			"session_id": s.ID,
			"duration_s": int(time.Since(startedAt).Seconds()),
		})
	}()

	ctx := r.Context()

	// ws → exec stdin: read from WebSocket and write to container
	go func() {
		defer s.Close()
		for {
			var msg termMsg
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				return
			}
			switch msg.Type {
			case "input":
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err != nil {
					// Try raw string as fallback (plain text input)
					data = []byte(msg.Data)
				}
				if _, err := s.RWC.Write(data); err != nil {
					return
				}
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					if err := h.runtime.ContainerExecResize(ctx, s.ExecID, uint(msg.Rows), uint(msg.Cols)); err != nil {
						slog.Debug("terminal: resize failed", "error", err)
					}
				}
			}
		}
	}()

	// exec stdout → ws: read from container and send to WebSocket
	buf := make([]byte, 4096)
	for {
		n, err := s.RWC.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			writeErr := wsjson.Write(ctx, conn, termMsg{Type: "output", Data: encoded})
			if writeErr != nil {
				return
			}
		}
		if err != nil {
			_ = wsjson.Write(ctx, conn, termMsg{Type: "closed", Message: "session ended"})
			return
		}
	}
}

