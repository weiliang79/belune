package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"

	"github.com/weiliang79/belune/internal/config"
	"github.com/weiliang79/belune/internal/service/backup"
	"github.com/weiliang79/belune/internal/store/generated"
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
// can show where control-plane backups are uploaded, and edit it. Credentials
// (access_key/secret_key) are never returned — write-only, same convention as
// project backup destinations.
type backupRemoteView struct {
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
	Bucket   string `json:"bucket"`
	Prefix   string `json:"prefix"`
	UseSSL   bool   `json:"use_ssl"`
}

func toBackupRemoteView(rc backup.RemoteConfig) backupRemoteView {
	return backupRemoteView{
		Endpoint: rc.Endpoint,
		Region:   rc.Region,
		Bucket:   rc.Bucket,
		Prefix:   rc.Prefix,
		UseSSL:   rc.UseSSL,
	}
}

type backupStatusView struct {
	LastSucceededAt *string           `json:"last_succeeded_at"`
	LastAttemptedAt *string           `json:"last_attempted_at"`
	LastError       *string           `json:"last_error"`
	RemoteEnabled   bool              `json:"remote_enabled"`
	Remote          *backupRemoteView `json:"remote"`
	Retention       map[string]any    `json:"retention"`
}

// ListBackupRuns returns backup runs most-recent-first, paginated via
// limit/offset query params (see parsePagination).
func (h *Handler) ListBackupRuns(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	runs, err := h.queries.ListBackupRuns(r.Context(), generated.ListBackupRunsParams{
		Limit:  limit,
		Offset: offset,
	})
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

// resolveBackupRetention reads the dashboard-editable retention settings
// fresh, falling back per-key to the .env-configured Config defaults when a
// setting is unset — mirrors the worker's copy of this resolution (see
// worker.resolveBackupRetention), kept separate since the two packages don't
// share a settings-reading helper.
func (h *Handler) resolveBackupRetention(ctx context.Context) (days, count int) {
	days, count = h.cfg.BackupRetainDays, h.cfg.BackupRetainCount
	if s, err := h.queries.GetSetting(ctx, config.SettingControlPlaneBackupRetainDays); err == nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(s.Value)); convErr == nil && n > 0 {
			days = n
		}
	}
	if s, err := h.queries.GetSetting(ctx, config.SettingControlPlaneBackupRetainCount); err == nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(s.Value)); convErr == nil && n > 0 {
			count = n
		}
	}
	return days, count
}

// GetBackupStatus returns a summary of the most recent backup activity.
func (h *Handler) GetBackupStatus(w http.ResponseWriter, r *http.Request) {
	rc := backup.LoadRemoteConfig(h.cfg)
	retainDays, retainCount := h.resolveBackupRetention(r.Context())
	status := backupStatusView{
		RemoteEnabled: rc.Enabled,
		Retention: map[string]any{
			"days":  retainDays,
			"count": retainCount,
		},
	}
	if rc.Enabled {
		v := toBackupRemoteView(rc)
		status.Remote = &v
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

// TestBackupRemote verifies the currently configured remote storage
// (dashboard-managed file, falling back to .env) is reachable and the bucket
// exists, without mutating anything. Read-only diagnostic for the admin
// Backups tab.
func (h *Handler) TestBackupRemote(w http.ResponseWriter, r *http.Request) {
	if !backup.LoadRemoteConfig(h.cfg).Enabled {
		writeError(w, http.StatusBadRequest,
			"remote storage is disabled — enable it from the Remote Storage card")
		return
	}
	if err := backup.New(h.cfg).Check(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type updateBackupRemoteRequest struct {
	Enabled   *bool  `json:"enabled"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	UseSSL    *bool  `json:"use_ssl"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// UpdateBackupRemote saves the dashboard-managed control-plane remote-storage
// config to cfg.BackupRemoteConfigPath (mode 0600), read fresh by the worker's
// S3 client and by scripts/backup.sh/belune-backup-upload on the next backup —
// no restart needed. Endpoint/region/bucket/prefix/use_ssl/enabled always
// reflect the submitted form state; a blank access_key or secret_key preserves
// the currently stored one (same convention as project backup destinations),
// so the dashboard never has to redisplay a saved secret to let the operator
// edit unrelated fields.
func (h *Handler) UpdateBackupRemote(w http.ResponseWriter, r *http.Request) {
	var req updateBackupRemoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	enabled := req.Enabled != nil && *req.Enabled
	bucket := strings.TrimSpace(req.Bucket)
	if enabled && bucket == "" {
		writeError(w, http.StatusBadRequest, "bucket is required to enable remote storage")
		return
	}

	current := backup.LoadRemoteConfig(h.cfg)
	next := backup.RemoteConfig{
		Enabled:   enabled,
		Endpoint:  strings.TrimSpace(req.Endpoint),
		Region:    strings.TrimSpace(req.Region),
		Bucket:    bucket,
		Prefix:    strings.TrimSpace(req.Prefix),
		UseSSL:    req.UseSSL == nil || *req.UseSSL,
		AccessKey: current.AccessKey,
		SecretKey: current.SecretKey,
	}
	if req.AccessKey != "" {
		next.AccessKey = req.AccessKey
	}
	if req.SecretKey != "" {
		next.SecretKey = req.SecretKey
	}

	if err := backup.SaveRemoteConfig(h.cfg.BackupRemoteConfigPath, next); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save remote storage config")
		return
	}

	h.audit(r, "update_backup_remote", "backup", "", nil)
	v := toBackupRemoteView(next)
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "remote": v})
}
