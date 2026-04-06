package eventwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/ws"
)

// Watcher monitors Docker container events and synchronizes application status.
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

	// Build map of running container app IDs
	running := make(map[string]bool)
	for _, c := range containers {
		appID, ok := c.Labels["application-id"]
		if !ok {
			continue
		}
		running[appID] = true
	}

	// Check all applications and reconcile status
	apps, err := w.queries.ListAllApplications(ctx)
	if err != nil {
		slog.Warn("eventwatcher: reconciliation failed to list applications", "error", err)
		return
	}

	for _, app := range apps {
		appIDStr := pgUUIDToString(app.ID)
		isRunning := running[appIDStr]

		if app.Status == status.ApplicationRunning && !isRunning {
			slog.Info("eventwatcher: reconciling stopped application", "app_id", appIDStr)
			w.updateAppStatus(ctx, app.ID, appIDStr, status.ApplicationStopped)
		} else if app.Status == status.ApplicationStopped && isRunning {
			slog.Info("eventwatcher: reconciling running application", "app_id", appIDStr)
			w.updateAppStatus(ctx, app.ID, appIDStr, status.ApplicationRunning)
		}
	}

	slog.Info("eventwatcher: reconciliation complete", "containers", len(containers), "apps", len(apps))
}

// watchEvents subscribes to Docker events and processes them.
func (w *Watcher) watchEvents(ctx context.Context) error {
	eventCh, errCh := w.runtime.ContainerEvents(ctx, map[string][]string{
		"label":  {"managed-by=selfhost-paas"},
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

// handleEvent processes a single Docker container event.
func (w *Watcher) handleEvent(ctx context.Context, event runtime.ContainerEvent) {
	appID, ok := event.Labels["application-id"]
	if !ok || appID == "" {
		return
	}

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

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
