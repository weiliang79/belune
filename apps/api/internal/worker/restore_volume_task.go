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

	"github.com/hibiken/asynq"

	"github.com/ungweiliang/selfhost-paas/internal/naming"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

// restoreVolumePayload restores an application volume from a previously-recorded
// backup. This is disruptive: the app is stopped, the volume is wiped and
// replaced with the snapshot, then the app is restarted.
type restoreVolumePayload struct {
	ApplicationVolumeID string `json:"application_volume_id"`
	BackupID            string `json:"backup_id"`
}

func (h *TaskHandler) HandleRestoreVolumeTask(ctx context.Context, t *asynq.Task) error {
	var payload restoreVolumePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal restore_volume payload: %w", err)
	}

	slog.Info("handling restore_volume task", "volume_id", payload.ApplicationVolumeID, "backup_id", payload.BackupID)

	volID, err := parseUUID(payload.ApplicationVolumeID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid application_volume_id (permanent): %w", err), asynq.SkipRetry)
	}
	backupID, err := parseUUID(payload.BackupID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid backup_id (permanent): %w", err), asynq.SkipRetry)
	}

	vol, err := h.Queries.GetApplicationVolume(ctx, volID)
	if err != nil {
		return errors.Join(fmt.Errorf("get volume (permanent): %w", err), asynq.SkipRetry)
	}
	bk, err := h.Queries.GetApplicationVolumeBackup(ctx, backupID)
	if err != nil {
		return errors.Join(fmt.Errorf("get backup (permanent): %w", err), asynq.SkipRetry)
	}
	if bk.ApplicationVolumeID != volID {
		return errors.Join(errors.New("backup does not belong to volume (permanent)"), asynq.SkipRetry)
	}

	appRow, err := h.Queries.GetApplicationWithProjectSlug(ctx, vol.ApplicationID)
	if err != nil {
		return errors.Join(fmt.Errorf("get application (permanent): %w", err), asynq.SkipRetry)
	}
	appIDStr := formatUUID(vol.ApplicationID)
	containerName := naming.ContainerName(appRow.ProjectSlug, appRow.Slug, appIDStr)
	volumeName := naming.AppVolumeName(appIDStr, vol.Name)

	archivePath, cleanup, err := h.resolveVolumeBackupFile(ctx, bk)
	if err != nil {
		return fmt.Errorf("resolve backup file: %w", err)
	}
	defer cleanup()

	if err := h.restoreAppVolume(ctx, volumeName, vol.MountPath, containerName, appRow.Status == "running", archivePath); err != nil {
		return err
	}

	slog.Info("volume restored", "volume_id", payload.ApplicationVolumeID, "backup_id", payload.BackupID)
	return nil
}

// resolveVolumeBackupFile returns a readable path to the backup archive,
// downloading the remote object to a temp file when there is no local copy. The
// returned cleanup removes any temp file.
func (h *TaskHandler) resolveVolumeBackupFile(ctx context.Context, bk generated.ApplicationVolumeBackup) (string, func(), error) {
	noop := func() {}
	if bk.LocalPath.Valid {
		if _, err := os.Stat(bk.LocalPath.String); err == nil {
			return bk.LocalPath.String, noop, nil
		}
	}
	if !bk.RemoteKey.Valid {
		return "", noop, errors.New("backup has no local file and no remote copy")
	}
	if !bk.BackupConfigID.Valid {
		return "", noop, errors.New("backup has no config to resolve its destination")
	}
	if h.BackupDestinations == nil {
		return "", noop, errors.New("backup destinations service is not configured")
	}

	client, err := h.BackupDestinations.ClientForVolumeBackupConfig(ctx, bk.BackupConfigID)
	if err != nil {
		return "", noop, fmt.Errorf("resolve backup destination: %w", err)
	}
	tmp := filepath.Join(os.TempDir(), "paas-volrestore-"+filepath.Base(bk.RemoteKey.String))
	cleanup := func() { _ = os.Remove(tmp) }
	if err := client.Download(ctx, bk.RemoteKey.String, tmp); err != nil {
		cleanup()
		return "", noop, err
	}
	return tmp, cleanup, nil
}

// restoreAppVolume stops the app (if running), wipes the named volume, untars
// the archive into it, then restarts the app. Stopping is mandatory: restoring
// under a live writer would corrupt the volume.
func (h *TaskHandler) restoreAppVolume(ctx context.Context, volumeName, mountPath, containerName string, appRunning bool, archivePath string) error {
	helperImage := h.Config.DatabaseBackupHelperImage
	if err := h.Runtime.PullImage(ctx, helperImage); err != nil {
		return fmt.Errorf("pull helper image: %w", err)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	if appRunning {
		if err := h.Runtime.StopContainer(ctx, containerName); err != nil {
			slog.Warn("restore volume: stop app (may already be stopped)", "container", containerName, "error", err)
		}
		defer func() {
			if err := h.Runtime.StartContainer(ctx, containerName); err != nil {
				slog.Error("restore volume: failed to restart app after restore", "container", containerName, "error", err)
			}
		}()
	}

	// mountPath is passed as the shell's positional $0 (a parameter, never
	// re-parsed as script text) so it cannot inject shell syntax even though it
	// originates from user-set volume config. `cd "$0"` then operates on the
	// mount dir directly.
	const script = `cd "$0" && find . -mindepth 1 -delete && tar xzf -`
	var stderr bytes.Buffer
	exit, err := h.Runtime.RunHelper(ctx, runtime.ContainerConfig{
		Image:   helperImage,
		Cmd:     []string{"sh", "-c", script, mountPath},
		Volumes: map[string]string{volumeName: mountPath},
	}, f, nil, &stderr)
	if err != nil {
		return fmt.Errorf("restore helper: %w", err)
	}
	if exit != 0 {
		return errors.Join(fmt.Errorf("restore tar exited %d: %s", exit, strings.TrimSpace(stderr.String())), asynq.SkipRetry)
	}
	return nil
}
