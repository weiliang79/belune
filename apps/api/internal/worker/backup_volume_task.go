package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/service/backup"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// backupVolumePayload triggers a snapshot of an application's persistent volume.
// MVP is config-based: BackupConfigID is always set (the config carries the
// destination + quiesce option).
type backupVolumePayload struct {
	ApplicationVolumeID string `json:"application_volume_id"`
	BackupConfigID      string `json:"backup_config_id"`
}

// HandleBackupVolumeTask tars an application volume and uploads it to the
// config's project destination. Hot by default (no downtime); when the config's
// quiesce flag is set and the app is running, the container is stopped for the
// duration of the tar to get a consistent snapshot.
func (h *TaskHandler) HandleBackupVolumeTask(ctx context.Context, t *asynq.Task) error {
	var payload backupVolumePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal backup_volume payload: %w", err)
	}

	volID, err := parseUUID(payload.ApplicationVolumeID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid application_volume_id (permanent): %w", err), asynq.SkipRetry)
	}
	vol, err := h.Queries.GetApplicationVolume(ctx, volID)
	if err != nil {
		return errors.Join(fmt.Errorf("get volume (permanent): %w", err), asynq.SkipRetry)
	}

	if payload.BackupConfigID == "" {
		return errors.Join(errors.New("backup_config_id is required (permanent)"), asynq.SkipRetry)
	}
	cid, err := parseUUID(payload.BackupConfigID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid backup_config_id (permanent): %w", err), asynq.SkipRetry)
	}
	cfg, err := h.Queries.GetApplicationVolumeBackupConfig(ctx, cid)
	if err != nil {
		return errors.Join(fmt.Errorf("get backup config (permanent): %w", err), asynq.SkipRetry)
	}
	if cfg.ApplicationVolumeID != volID {
		return errors.Join(errors.New("backup config does not belong to volume (permanent)"), asynq.SkipRetry)
	}
	if h.BackupDestinations == nil {
		return errors.Join(errors.New("backup destinations service is not configured (permanent)"), asynq.SkipRetry)
	}
	dest, err := h.BackupDestinations.Resolve(ctx, cfg.DestinationID)
	if err != nil {
		return errors.Join(fmt.Errorf("resolve destination (permanent): %w", err), asynq.SkipRetry)
	}
	destClient, err := backup.NewDestinationClient(dest)
	if err != nil {
		return errors.Join(fmt.Errorf("build destination client (permanent): %w", err), asynq.SkipRetry)
	}

	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, vol.ApplicationID)
	if err != nil {
		return errors.Join(fmt.Errorf("get application (permanent): %w", err), asynq.SkipRetry)
	}
	appIDStr := formatUUID(vol.ApplicationID)
	containerName := naming.ContainerName(appRow.ProjectSlug, appRow.Slug, appIDStr)
	volumeName := naming.AppVolumeName(appIDStr, vol.Name)

	// Record the run up front so the UI shows a "running" backup immediately.
	run, err := h.Queries.InsertApplicationVolumeBackup(ctx, generated.InsertApplicationVolumeBackupParams{
		ApplicationVolumeID: volID,
		BackupConfigID:      cid,
	})
	if err != nil {
		return fmt.Errorf("insert backup run: %w", err)
	}

	lg := &runLog{}
	lg.step("Volume backup started (volume=%s, path=%s, quiesce=%t)", vol.Name, vol.MountPath, cfg.Quiesce)

	if err := os.MkdirAll(h.Config.DatabaseBackupDir, 0o755); err != nil {
		h.failVolumeBackupLog(ctx, run.ID, fmt.Sprintf("create backup dir: %v", err), lg)
		return fmt.Errorf("create backup dir: %w", err)
	}
	fileName := fmt.Sprintf("%s-%s-%s.tar.gz", appRow.Slug, vol.Name, time.Now().UTC().Format("20060102T150405Z"))
	localPath := filepath.Join(h.Config.DatabaseBackupDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		h.failVolumeBackupLog(ctx, run.ID, fmt.Sprintf("create backup file: %v", err), lg)
		return fmt.Errorf("create backup file: %w", err)
	}

	snapErr := h.snapshotAppVolume(ctx, volumeName, vol.MountPath, containerName, cfg.Quiesce, appRow.Status == "running", f, lg)
	closeErr := f.Close()
	if snapErr != nil {
		_ = os.Remove(localPath)
		h.failVolumeBackupLog(ctx, run.ID, snapErr.Error(), lg)
		return snapErr
	}
	if closeErr != nil {
		_ = os.Remove(localPath)
		h.failVolumeBackupLog(ctx, run.ID, fmt.Sprintf("finalise backup file: %v", closeErr), lg)
		return fmt.Errorf("finalise backup file: %w", closeErr)
	}

	var sizeBytes int64
	if info, statErr := os.Stat(localPath); statErr == nil {
		sizeBytes = info.Size()
	}
	lg.step("Archive written: %s (%d bytes)", fileName, sizeBytes)

	// Object key = <destination.prefix>/<config.prefix>/<file>.
	key := backup.BuildKey(dest.Prefix, backup.BuildKey(cfg.Prefix, fileName))
	lg.step("Uploading to destination: %s", key)
	if _, upErr := destClient.UploadTo(ctx, localPath, key); upErr != nil {
		_ = os.Remove(localPath)
		h.failVolumeBackupLog(ctx, run.ID, fmt.Sprintf("upload to destination: %v", upErr), lg)
		return fmt.Errorf("upload to destination: %w", upErr)
	}
	// Remote is authoritative; drop the local copy so the server disk doesn't
	// mirror the bucket.
	removeLocalAfterUpload(localPath, lg)
	lg.step("Backup succeeded")

	h.finaliseVolumeBackup(ctx, run.ID, generated.UpdateApplicationVolumeBackupParams{
		ID:         run.ID,
		FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     "succeeded",
		RemoteKey:  pgtype.Text{String: key, Valid: true},
		SizeBytes:  sizeBytes,
		Log:        pgtype.Text{String: lg.String(), Valid: true},
	})
	if err := h.Queries.SetApplicationVolumeBackupConfigLastRun(ctx, generated.SetApplicationVolumeBackupConfigLastRunParams{
		ID:        cid,
		LastRunAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		slog.Warn("backup_volume: failed to stamp last_run_at", "config_id", formatUUID(cid), "error", err)
	}

	slog.Info("volume backed up", "volume_id", payload.ApplicationVolumeID, "size_bytes", sizeBytes, "key", key)
	return nil
}

// snapshotAppVolume tars the named volume's contents (mounted at mountPath in a
// short-lived helper) into f. When quiesce is requested and the app is running,
// the app container is stopped for the tar and restarted afterwards.
func (h *TaskHandler) snapshotAppVolume(ctx context.Context, volumeName, mountPath, containerName string, quiesce, appRunning bool, f *os.File, lg *runLog) error {
	helperImage := h.Config.DatabaseBackupHelperImage
	if err := h.Runtime.PullImage(ctx, helperImage); err != nil {
		return fmt.Errorf("pull helper image: %w", err)
	}

	if quiesce && appRunning {
		lg.step("Stopping application for consistent (quiesced) snapshot")
		if err := h.Runtime.StopContainer(ctx, containerName); err != nil {
			slog.Warn("snapshot volume: stop app (may already be stopped)", "container", containerName, "error", err)
		}
		defer func() {
			if err := h.Runtime.StartContainer(ctx, containerName); err != nil {
				slog.Error("snapshot volume: failed to restart app after backup", "container", containerName, "error", err)
			}
			lg.step("Application restarted")
		}()
	} else if quiesce {
		lg.step("Quiesce requested but app is not running — snapshotting directly")
	}

	var stderr bytes.Buffer
	exit, err := h.Runtime.RunHelper(ctx, runtime.ContainerConfig{
		Image:   helperImage,
		Cmd:     []string{"tar", "czf", "-", "-C", mountPath, "."},
		Volumes: map[string]string{volumeName: mountPath},
	}, nil, f, &stderr)
	lg.raw(stderr.String())
	if err != nil {
		return fmt.Errorf("snapshot helper: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("snapshot tar exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}
	lg.step("Volume snapshot completed")
	return nil
}

func (h *TaskHandler) failVolumeBackupLog(ctx context.Context, id pgtype.UUID, errMsg string, lg *runLog) {
	slog.Error("volume backup failed", "backup_id", formatUUID(id), "error", errMsg)
	logText := ""
	if lg != nil {
		lg.step("Backup failed: %s", errMsg)
		logText = lg.String()
	}
	h.finaliseVolumeBackup(ctx, id, generated.UpdateApplicationVolumeBackupParams{
		ID:         id,
		FinishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     "failed",
		Error:      pgtype.Text{String: errMsg, Valid: true},
		Log:        pgtype.Text{String: logText, Valid: true},
	})
}

func (h *TaskHandler) finaliseVolumeBackup(ctx context.Context, id pgtype.UUID, params generated.UpdateApplicationVolumeBackupParams) {
	if !id.Valid {
		return
	}
	if err := h.Queries.UpdateApplicationVolumeBackup(ctx, params); err != nil {
		slog.Warn("backup_volume: failed to update run record", "backup_id", formatUUID(id), "error", err)
	}
}
