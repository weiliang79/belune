package eventwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/runtime"
	"github.com/weiliang79/belune/internal/status"
	"github.com/weiliang79/belune/internal/store/generated"
	"github.com/weiliang79/belune/internal/ws"
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
		} else if (app.Status == status.ApplicationStopped || app.Status == status.ApplicationError) && isRunning {
			// "error" is included because the container is the ground truth: if
			// it is up, the application is running, whatever went wrong before.
			// Without this an app could sit in "error" indefinitely while
			// serving traffic — reconciliation used to only ever move between
			// running and stopped, which was complete only while "error" was
			// never written.
			slog.Info("eventwatcher: reconciling running application", "app_id", appIDStr, "from", app.Status)
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
		"label": {"managed-by=belune"},
		"event": {"start", "stop", "die", "restart", "oom"},
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

	newStatus, ok := ApplicationStatusForEvent(event)
	if !ok {
		return
	}
	if newStatus == status.ApplicationError && event.Status == "oom" {
		slog.Warn("eventwatcher: container killed by OOM", "app_id", appID, "container", event.ContainerName)
	}

	// Only a `die` needs the current status to be interpreted; every other
	// event stands on its own, so the read is skipped for them.
	if event.Status == "die" {
		current, err := w.queries.GetApplication(ctx, appUUID)
		if err == nil && !ApplyDieStatus(newStatus, current.Status) {
			slog.Debug("eventwatcher: keeping existing status",
				"app_id", appID, "keeping", current.Status, "ignored", newStatus)
			return
		}
	}

	slog.Info("eventwatcher: container event",
		"app_id", appID,
		"event", event.Status,
		"container", event.ContainerName,
		"new_status", newStatus,
	)

	w.updateAppStatus(ctx, appUUID, appID, newStatus)
}

// ApplicationStatusForEvent maps a Docker container event to an application
// status, returning ok=false for events that should not change status.
//
// A crash and a clean stop are distinguished by exit code, so an app that
// crash-loops no longer looks identical to one the user deliberately stopped.
func ApplicationStatusForEvent(event runtime.ContainerEvent) (newStatus string, ok bool) {
	switch event.Status {
	case "start", "restart":
		return status.ApplicationRunning, true
	case "stop":
		return status.ApplicationStopped, true
	case "die":
		return exitStatus(event.Labels["exitCode"]), true
	case "oom":
		return status.ApplicationError, true
	default:
		return "", false
	}
}

// ApplyDieStatus reports whether the status derived from a `die` event should
// be written over the status the application already has.
//
// A `die` is ambiguous: it carries an exit code but says nothing about who
// asked for the exit, and it races with the writes made by whoever did the
// asking. A `stop` event is not ambiguous — Docker emits it only because
// something asked the container to stop — so callers apply those unconditionally
// and never consult this.
//
// That distinction is the fix for the bug where stopping an application left it
// showing "error". Docker emits `die` then `stop`; an earlier version guarded on
// the status alone, so a stop whose process exited non-zero wrote "error" on the
// die, and then the very event that would have corrected it was swallowed for
// looking like a downgrade. The application stayed errored indefinitely.
func ApplyDieStatus(derived, current string) bool {
	// Never downgrade a failed deploy to "stopped". The compensating cleanup
	// removes the half-created container, and that die can land after
	// failDeployment has already recorded the error.
	if derived == status.ApplicationStopped && current == status.ApplicationError {
		return false
	}
	// Never turn a deliberate stop into an error. An application already
	// recorded as stopped has nothing left running to crash, so this die is
	// reporting the stop that was asked for — whatever exit code came back.
	//
	// Wrapper commands make that code untrustworthy in exactly this case:
	// `npm start` exits 1 when its child is terminated by SIGTERM instead of
	// propagating 143, so the exit code alone cannot tell a stop from a crash.
	if derived == status.ApplicationError && current == status.ApplicationStopped {
		return false
	}
	return true
}

// exitStatus decides whether an exit code describes a crash.
//
// Being killed by a signal is not a crash. `docker stop` sends SIGTERM, and a
// process that does not install a handler is terminated by it — which Docker
// reports as exit 128+signal, so 143 for SIGTERM and 137 for SIGKILL after the
// grace period. Plenty of application images never trap SIGTERM, so treating
// every non-zero exit as a crash marked ordinary, deliberate stops as errors.
//
// The database mapping gets away with the simpler rule because Postgres and
// MySQL do trap SIGTERM and exit 0. Generalising that to arbitrary application
// images was the mistake this corrects.
//
// An OOM kill also exits 137, but Docker emits a distinct `oom` event first and
// the caller refuses to downgrade an errored application back to stopped, so
// the more informative status survives.
//
// What remains an error is an exit code the application chose itself: a failed
// start, a fatal config error, a panic.
func exitStatus(exitCode string) string {
	switch exitCode {
	case "", "0":
		return status.ApplicationStopped
	}
	code, err := strconv.Atoi(exitCode)
	if err != nil {
		// Unparseable: prefer the non-alarming reading rather than inventing a
		// failure the user cannot explain.
		return status.ApplicationStopped
	}
	if code > 128 && code <= 128+64 {
		return status.ApplicationStopped
	}
	return status.ApplicationError
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
