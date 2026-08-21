package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// replaceDatabasePayload rebuilds a database that was deleted, from the
// tombstone left behind, and restores one of its surviving backups into it.
type replaceDatabasePayload struct {
	DatabaseID string `json:"database_id"`
	BackupID   string `json:"backup_id"`
}

// HandleReplaceDatabaseTask provisions a recreated database and then restores a
// backup into it.
//
// The database row already exists — the handler recreated it from the tombstone
// synchronously, under the original slug and credentials, so the UI shows it
// immediately and the container comes up on the hostname dependent applications
// are already configured for. All this task does is bring the container up and
// hand off.
//
// The hand-off is a separate task rather than an inlined restore because the
// restore path is the same one an ordinary restore takes, and having exactly one
// of those is worth more than saving a queue hop. If the enqueue fails the
// database is up and empty, which is visible and recoverable by restoring again
// from its Backups tab — the failure a user can see and undo.
func (h *TaskHandler) HandleReplaceDatabaseTask(ctx context.Context, t *asynq.Task) error {
	var payload replaceDatabasePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("unmarshal replace_database payload: %w", err), asynq.SkipRetry)
	}

	slog.Info("handling replace_database task", "database_id", payload.DatabaseID, "backup_id", payload.BackupID)

	dbID, err := parseUUID(payload.DatabaseID)
	if err != nil {
		return errors.Join(fmt.Errorf("invalid database_id (permanent): %w", err), asynq.SkipRetry)
	}

	db, err := h.Queries.GetDatabase(ctx, dbID)
	if err != nil {
		return fmt.Errorf("get database: %w", err)
	}

	rt, err := h.runtimeForDatabase(ctx, dbID)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("resolve server: %v", err))
		return err
	}

	creds, err := h.decryptDBCredentials(db)
	if err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("credentials: %v", err))
		return errors.Join(fmt.Errorf("credentials (permanent): %w", err), asynq.SkipRetry)
	}

	// External access is not carried across: the tombstone does not record the
	// loopback port, and a port allocated to a database that no longer exists
	// would be misleading. The toggle can be turned back on afterwards.
	if err := h.provisionDBContainer(ctx, rt, db, creds, 0); err != nil {
		h.failDatabase(ctx, dbID, err.Error())
		return err
	}

	// The engine accepts connections a moment after the container starts;
	// restoring before then fails for no reason anyone can act on.
	if err := h.waitForDBReady(ctx, rt, db, creds); err != nil {
		h.failDatabase(ctx, dbID, fmt.Sprintf("replacement not ready: %v", err))
		return err
	}

	restorePayload, err := json.Marshal(restoreDBPayload{
		DatabaseID: payload.DatabaseID,
		BackupID:   payload.BackupID,
	})
	if err != nil {
		return fmt.Errorf("marshal restore payload: %w", err)
	}
	if _, err := h.Enqueuer.Enqueue(asynq.NewTask(TypeRestoreDB, restorePayload), asynq.Queue("critical")); err != nil {
		// The database is up and empty. Deliberately not a failure of the
		// database itself: it is running and reachable, and the data is one
		// restore away from its own Backups tab.
		slog.Error("replacement provisioned but restore could not be enqueued",
			"database_id", payload.DatabaseID, "backup_id", payload.BackupID, "error", err)
		return fmt.Errorf("enqueue restore for replacement: %w", err)
	}

	slog.Info("replacement database provisioned; restore enqueued",
		"database_id", payload.DatabaseID, "backup_id", payload.BackupID)
	return nil
}
