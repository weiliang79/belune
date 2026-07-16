package handler

import (
	"log/slog"
	"net/http"
)

// maintenanceQueues are the Asynq queues surfaced/cleared by the Maintenance
// section. Matches the queues configured in internal/worker/worker.go.
var maintenanceQueues = []string{"critical", "default", "low"}

type queueDepth struct {
	Queue    string `json:"queue"`
	Pending  int    `json:"pending"`
	Active   int    `json:"active"`
	Retry    int    `json:"retry"`
	Archived int    `json:"archived"`
}

type queueStatusResponse struct {
	Queues     []queueDepth `json:"queues"`
	TotalStuck int          `json:"total_stuck"` // archived + retry across all queues
}

// GetQueueStatus reports per-queue depth for the admin Maintenance section.
// GET /api/maintenance/queue (admin only)
func (h *Handler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	if h.inspector == nil {
		writeError(w, http.StatusServiceUnavailable, "queue inspector unavailable")
		return
	}
	out := queueStatusResponse{Queues: make([]queueDepth, 0, len(maintenanceQueues))}
	for _, q := range maintenanceQueues {
		info, err := h.inspector.GetQueueInfo(q)
		if err != nil {
			// Queue may not exist yet (no task ever enqueued) — treat as empty.
			continue
		}
		out.Queues = append(out.Queues, queueDepth{
			Queue:    q,
			Pending:  info.Pending,
			Active:   info.Active,
			Retry:    info.Retry,
			Archived: info.Archived,
		})
		out.TotalStuck += info.Retry + info.Archived
	}
	writeJSON(w, http.StatusOK, out)
}

// ClearQueue deletes stuck tasks — archived (dead-letter) + retry — across all
// queues. It never touches pending or active tasks, so an in-flight deploy is
// never killed. POST /api/maintenance/queue/clear (admin only)
func (h *Handler) ClearQueue(w http.ResponseWriter, r *http.Request) {
	if h.inspector == nil {
		writeError(w, http.StatusServiceUnavailable, "queue inspector unavailable")
		return
	}
	cleared := 0
	for _, q := range maintenanceQueues {
		if n, err := h.inspector.DeleteAllArchivedTasks(q); err == nil {
			cleared += n
		} else {
			slog.Warn("clear queue: delete archived tasks failed", "queue", q, "error", err)
		}
		if n, err := h.inspector.DeleteAllRetryTasks(q); err == nil {
			cleared += n
		} else {
			slog.Warn("clear queue: delete retry tasks failed", "queue", q, "error", err)
		}
	}
	h.audit(r, "clear_job_queue", "queue", "", map[string]any{"cleared": cleared})
	writeJSON(w, http.StatusOK, map[string]int{"cleared": cleared})
}

// ClearPendingQueue deletes pending (queued-but-not-started) tasks across all
// queues. Active (in-flight) tasks are never touched, so a running deploy
// survives. This is the "cancel the backlog" action, distinct from ClearQueue's
// "remove stuck jobs". POST /api/maintenance/queue/clear-pending (admin only)
func (h *Handler) ClearPendingQueue(w http.ResponseWriter, r *http.Request) {
	if h.inspector == nil {
		writeError(w, http.StatusServiceUnavailable, "queue inspector unavailable")
		return
	}
	cleared := 0
	for _, q := range maintenanceQueues {
		if n, err := h.inspector.DeleteAllPendingTasks(q); err == nil {
			cleared += n
		} else {
			slog.Warn("clear pending queue: delete pending tasks failed", "queue", q, "error", err)
		}
	}
	h.audit(r, "clear_pending_queue", "queue", "", map[string]any{"cleared": cleared})
	writeJSON(w, http.StatusOK, map[string]int{"cleared": cleared})
}
