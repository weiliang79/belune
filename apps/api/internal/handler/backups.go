package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"

	"github.com/ungweiliang/selfhost-paas/internal/worker"
)

type backupRunView struct {
	ID         string  `json:"id"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	Status     string  `json:"status"`
	RemoteKey  *string `json:"remote_key"`
	SizeBytes  int64   `json:"size_bytes"`
	Error      *string `json:"error"`
}

type backupStatusView struct {
	LastSucceededAt *string        `json:"last_succeeded_at"`
	LastAttemptedAt *string        `json:"last_attempted_at"`
	LastError       *string        `json:"last_error"`
	RemoteEnabled   bool           `json:"remote_enabled"`
	Retention       map[string]any `json:"retention"`
}

// ListBackupRuns returns the 20 most recent backup runs.
func (h *Handler) ListBackupRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.queries.ListBackupRuns(r.Context(), 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backup runs")
		return
	}

	views := make([]backupRunView, 0, len(runs))
	for _, run := range runs {
		v := backupRunView{
			ID:        uuidToString(run.ID),
			StartedAt: run.StartedAt.Time.Format(time.RFC3339),
			Status:    run.Status,
			SizeBytes: run.SizeBytes,
		}
		if run.FinishedAt.Valid {
			s := run.FinishedAt.Time.Format(time.RFC3339)
			v.FinishedAt = &s
		}
		if run.RemoteKey.Valid {
			v.RemoteKey = &run.RemoteKey.String
		}
		if run.Error.Valid {
			v.Error = &run.Error.String
		}
		views = append(views, v)
	}

	writeJSON(w, http.StatusOK, views)
}

// GetBackupStatus returns a summary of the most recent backup activity.
func (h *Handler) GetBackupStatus(w http.ResponseWriter, r *http.Request) {
	status := backupStatusView{
		RemoteEnabled: h.cfg.BackupRemoteEnabled,
		Retention: map[string]any{
			"days":  h.cfg.BackupRetainDays,
			"count": h.cfg.BackupRetainCount,
		},
	}

	if last, err := h.queries.GetLastBackupRun(r.Context()); err == nil {
		s := last.StartedAt.Time.Format(time.RFC3339)
		status.LastAttemptedAt = &s
		if last.Error.Valid {
			status.LastError = &last.Error.String
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to query backup status")
		return
	}

	if succeeded, err := h.queries.GetLastSucceededBackupRun(r.Context()); err == nil {
		if succeeded.FinishedAt.Valid {
			s := succeeded.FinishedAt.Time.Format(time.RFC3339)
			status.LastSucceededAt = &s
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to query backup status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// TriggerBackupRun enqueues a TypeBackupNow task.
func (h *Handler) TriggerBackupRun(w http.ResponseWriter, r *http.Request) {
	// Reject if a backup is already in progress to avoid concurrent archive writes.
	last, err := h.queries.GetLastBackupRun(r.Context())
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to check backup status")
		return
	}
	if err == nil && last.Status == "running" {
		writeError(w, http.StatusConflict, "a backup is already in progress")
		return
	}

	task := asynq.NewTask(worker.TypeBackupNow, nil)
	if _, err := h.asynq.Enqueue(task, asynq.Queue("low")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue backup task")
		return
	}

	h.audit(r, "backup_triggered", "backup", "", nil)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}
