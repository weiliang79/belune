package eventwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiling79/belune/internal/runtime"
	"github.com/weiling79/belune/internal/status"
	"github.com/weiling79/belune/internal/store/generated"
	"github.com/weiling79/belune/internal/ws"
)

const (
	labelApplicationID = "application-id"
	labelDatabaseID    = "database-id"
)

// Watcher monitors Docker container events and synchronizes application and
// database status with actual container state.
type Watcher struct {
	runtime     runtime.ContainerRuntime
	queries     *generated.Queries
	broadcaster *ws.ContainerStatusBroadcaster
}

func New(rt runtime.ContainerRuntime, queries *generated.Queries, broadcaster *ws.ContainerStatusBroadcaster) *Watcher {
	return &Watcher{
		runtime:     rt,
		queries:     queries,
		broadcaster: broadcaster,
	}
}

// Run starts the event watcher. It first reconciles current state, then
// listens for Docker events. On error it reconnects with backoff.
func (w *Watcher) Run(ctx context.Context) {
	// Initial reconciliation
	w.reconcile(ctx)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := w.watchEvents(ctx)
		if ctx.Err() != nil {
			return
		}

		slog.Warn("eventwatcher: event stream ended, reconnecting", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Re-reconcile on reconnect
		w.reconcile(ctx)

		backoff = min(backoff*2, maxBackoff)
	}
}

// reconcile compares running containers with DB status and fixes mismatches.
func (w *Watcher) reconcile(ctx context.Context) {
	containers, err := w.runtime.ListContainers(ctx)
	if err != nil {
		slog.Warn("eventwatcher: reconciliation failed to list containers", "error", err)
		return
	}

	// Build maps of running container app/database IDs. Databases are also keyed
	// by container name (which equals the slug) so databases provisioned before
	// the database-id label existed are still recognized as running and not
	// falsely flipped to stopped.
	runningApps := make(map[string]bool)
	runningDBs := make(map[string]bool)
	runningNames := make(map[string]bool)
	for _, c := range containers {
		runningNames[c.Name] = true
		if appID, ok := c.Labels[labelApplicationID]; ok {
			runningApps[appID] = true
		}
		if dbID, ok := c.Labels[labelDatabaseID]; ok {
			runningDBs[dbID] = true
		}
	}

	// Check all applications and reconcile status
	apps, err := w.queries.ListAllApplications(ctx)
	if err != nil {
		slog.Warn("eventwatcher: reconciliation failed to list applications", "error", err)
		return
	}

	for _, app := range apps {
		appIDStr := pgUUIDToString(app.ID)
		isRunning := runningApps[appIDStr]

		if app.Status == status.ApplicationRunning && !isRunning {
			slog.Info("eventwatcher: reconciling stopped application", "app_id", appIDStr)
			w.updateAppStatus(ctx, app.ID, appIDStr, status.ApplicationStopped)
		} else if app.Status == status.ApplicationStopped && isRunning {
			slog.Info("eventwatcher: reconciling running application", "app_id", appIDStr)
			w.updateAppStatus(ctx, app.ID, appIDStr, status.ApplicationRunning)
		}
	}

	// Check all databases and reconcile status. Only the steady running/stopped
	// states are reconciled; transient states (creating, upgrading, backing_up,
	// failed) are left for their owning task to resolve.
	dbs, err := w.queries.ListAllDatabases(ctx)
	if err != nil {
		slog.Warn("eventwatcher: reconciliation failed to list databases", "error", err)
		return
	}

	for _, db := range dbs {
		dbIDStr := pgUUIDToString(db.ID)
		isRunning := runningDBs[dbIDStr] || runningNames[db.Slug]

		if db.Status == status.DatabaseRunning && !isRunning {
			slog.Info("eventwatcher: reconciling stopped database", "database_id", dbIDStr)
			w.updateDatabaseStatus(ctx, db.ID, dbIDStr, status.DatabaseStopped)
		} else if db.Status == status.DatabaseStopped && isRunning {
			slog.Info("eventwatcher: reconciling running database", "database_id", dbIDStr)
			w.updateDatabaseStatus(ctx, db.ID, dbIDStr, status.DatabaseRunning)
		}
	}

	slog.Info("eventwatcher: reconciliation complete", "containers", len(containers), "apps", len(apps), "databases", len(dbs))
}

// watchEvents subscribes to Docker events and processes them.
func (w *Watcher) watchEvents(ctx context.Context) error {
	eventCh, errCh := w.runtime.ContainerEvents(ctx, map[string][]string{
		"label":  {"managed-by=belune"},
		"event":  {"start", "stop", "die", "restart", "oom"},
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errCh:
			return err

		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			w.handleEvent(ctx, event)
		}
	}
}

// handleEvent processes a single Docker container event, dispatching to the
// application or database handler based on which managed-resource label the
// container carries.
func (w *Watcher) handleEvent(ctx context.Context, event runtime.ContainerEvent) {
	if appID := event.Labels[labelApplicationID]; appID != "" {
		w.handleApplicationEvent(ctx, event, appID)
		return
	}
	if dbID := event.Labels[labelDatabaseID]; dbID != "" {
		w.handleDatabaseEvent(ctx, event, dbID)
		return
	}
}

func (w *Watcher) handleApplicationEvent(ctx context.Context, event runtime.ContainerEvent, appID string) {
	var appUUID pgtype.UUID
	if err := appUUID.Scan(appID); err != nil {
		slog.Debug("eventwatcher: invalid application-id label", "value", appID)
		return
	}

	var newStatus string
	switch event.Status {
	case "start", "restart":
		newStatus = status.ApplicationRunning
	case "stop":
		newStatus = status.ApplicationStopped
	case "die":
		newStatus = status.ApplicationStopped
	case "oom":
		newStatus = status.ApplicationStopped
		slog.Warn("eventwatcher: container killed by OOM", "app_id", appID, "container", event.ContainerName)
	default:
		return
	}

	slog.Info("eventwatcher: container event",
		"app_id", appID,
		"event", event.Status,
		"container", event.ContainerName,
		"new_status", newStatus,
	)

	w.updateAppStatus(ctx, appUUID, appID, newStatus)
}

// databaseStatusForEvent maps a Docker container event to a database status,
// returning ok=false for events that should not change status.
func databaseStatusForEvent(event runtime.ContainerEvent) (newStatus string, ok bool) {
	switch event.Status {
	case "start", "restart":
		return status.DatabaseRunning, true
	case "stop":
		return status.DatabaseStopped, true
	case "die":
		// A clean stop exits 0; a non-zero exit means the container crashed
		// (e.g. a misconfigured image), which we surface as failed.
		if event.Labels["exitCode"] == "0" {
			return status.DatabaseStopped, true
		}
		return status.DatabaseFailed, true
	case "oom":
		return status.DatabaseFailed, true
	default:
		return "", false
	}
}

func (w *Watcher) handleDatabaseEvent(ctx context.Context, event runtime.ContainerEvent, dbID string) {
	var dbUUID pgtype.UUID
	if err := dbUUID.Scan(dbID); err != nil {
		slog.Debug("eventwatcher: invalid database-id label", "value", dbID)
		return
	}

	newStatus, ok := databaseStatusForEvent(event)
	if !ok {
		return
	}
	if event.Status == "oom" {
		slog.Warn("eventwatcher: database container killed by OOM", "database_id", dbID, "container", event.ContainerName)
	}

	slog.Info("eventwatcher: database container event",
		"database_id", dbID,
		"event", event.Status,
		"container", event.ContainerName,
		"new_status", newStatus,
	)

	w.updateDatabaseStatus(ctx, dbUUID, dbID, newStatus)
}

func (w *Watcher) updateAppStatus(ctx context.Context, appUUID pgtype.UUID, appIDStr, newStatus string) {
	_, err := w.queries.UpdateApplicationStatus(ctx, generated.UpdateApplicationStatusParams{
		ID:     appUUID,
		Status: newStatus,
	})
	if err != nil {
		slog.Warn("eventwatcher: failed to update app status", "app_id", appIDStr, "error", err)
		return
	}

	if w.broadcaster != nil {
		w.broadcaster.BroadcastStatus(appIDStr, newStatus)
	}
}

func (w *Watcher) updateDatabaseStatus(ctx context.Context, dbUUID pgtype.UUID, dbIDStr, newStatus string) {
	_, err := w.queries.UpdateDatabaseStatus(ctx, generated.UpdateDatabaseStatusParams{
		ID:     dbUUID,
		Status: newStatus,
	})
	if err != nil {
		slog.Warn("eventwatcher: failed to update database status", "database_id", dbIDStr, "error", err)
		return
	}

	if w.broadcaster != nil {
		w.broadcaster.BroadcastDatabaseStatus(dbIDStr, newStatus)
	}
}

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
