package handler

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"

	"github.com/weiliang79/belune/internal/service/backup"
	"github.com/weiliang79/belune/internal/worker"
)

type backupRunView struct {
	ID         string  `json:"id"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	Status     string  `json:"status"`
	RemoteKey  *string `json:"remote_key"`
	SizeBytes  int64   `json:"size_bytes"`
	Error      *string `json:"error"`
	Log        string  `json:"log"`
}

// backupRemoteView exposes the non-secret remote-storage config so the admin UI
// can show where control-plane backups are uploaded. Credentials are never
// returned — remote storage is configured via BACKUP_S3_* env vars in .env.
type backupRemoteView struct {
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
	Bucket   string `json:"bucket"`
	Prefix   string `json:"prefix"`
	UseSSL   bool   `json:"use_ssl"`
}

type backupStatusView struct {
	LastSucceededAt *string           `json:"last_succeeded_at"`
	LastAttemptedAt *string           `json:"last_attempted_at"`
	LastError       *string           `json:"last_error"`
	RemoteEnabled   bool              `json:"remote_enabled"`
	Remote          *backupRemoteView `json:"remote"`
	Retention       map[string]any    `json:"retention"`
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
			Log:       run.Log,
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
	if h.cfg.BackupRemoteEnabled {
		status.Remote = &backupRemoteView{
			Endpoint: h.cfg.BackupS3Endpoint,
			Region:   h.cfg.BackupS3Region,
			Bucket:   h.cfg.BackupS3Bucket,
			Prefix:   h.cfg.BackupS3Prefix,
			UseSSL:   h.cfg.BackupS3UseSSL,
		}
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

	// Fail fast when the backup script is missing so the user gets immediate
	// feedback instead of an enqueued task that always fails in the worker.
	scriptPath := h.cfg.BackupScriptPath
	if scriptPath == "" {
		scriptPath = "/opt/belune/scripts/backup.sh"
	}
	if _, statErr := os.Stat(scriptPath); errors.Is(statErr, os.ErrNotExist) {
		writeError(w, http.StatusBadRequest,
			"backup script not found at "+scriptPath+" — this host is not configured for control-plane backups")
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

// TestBackupRemote verifies the env-configured remote storage is reachable and
// the bucket exists, without mutating anything. Read-only diagnostic for the
// admin Backups tab.
func (h *Handler) TestBackupRemote(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.BackupRemoteEnabled {
		writeError(w, http.StatusBadRequest,
			"remote storage is disabled — set BACKUP_REMOTE_ENABLED and BACKUP_S3_* in .env")
		return
	}
	if err := backup.New(h.cfg).Check(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
