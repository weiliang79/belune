// Package logcollector attaches to running container log streams, batch-inserts
// log lines into the application_logs table, and publishes them to Redis pub/sub
// so live SSE consumers can receive them.
package logcollector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

const labelApplicationID = "application-id"

// maxConcurrentWatchers caps how many container log streams the collector
// will track concurrently. Each watcher holds a Docker log stream + a
// goroutine, so an unbounded set could exhaust file descriptors on a host
// running hundreds of containers. The cap is intentionally generous: typical
// single-host PaaS instances run dozens of apps, not hundreds.
const maxConcurrentWatchers = 200

// Collector watches running containers and persists their logs.
type Collector struct {
	runtime    runtime.ContainerRuntime
	queries    *generated.Queries
	rdb        *redis.Client
	mu         sync.Mutex
	watchers   map[string]context.CancelFunc // containerID → cancel
	maxWatch   int
	batchSize  int
	flushEvery time.Duration
}

// New creates a new log collector.
func New(rt runtime.ContainerRuntime, queries *generated.Queries, rdb *redis.Client) *Collector {
	return &Collector{
		runtime:    rt,
		queries:    queries,
		rdb:        rdb,
		watchers:   make(map[string]context.CancelFunc),
		maxWatch:   maxConcurrentWatchers,
		batchSize:  50,
		flushEvery: 2 * time.Second,
	}
}

// Run starts the collector, blocking until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	slog.Info("application log collector starting")
	c.sync(ctx)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			for _, cancel := range c.watchers {
				cancel()
			}
			c.mu.Unlock()
			return
		case <-ticker.C:
			c.sync(ctx)
		}
	}
}

// sync lists running containers and starts/stops watchers as needed.
// For newly-attached containers, it picks up log collection from the last
// recorded_at for that application (or from the beginning if none).
func (c *Collector) sync(ctx context.Context) {
	containers, err := c.runtime.ListContainers(ctx)
	if err != nil {
		slog.Warn("log collector: failed to list containers", "error", err)
		return
	}

	// Build set of running container IDs that have our app label.
	active := make(map[string]struct{})
	for _, ctr := range containers {
		if ctr.Status != "running" {
			continue
		}
		appID, ok := ctr.Labels[labelApplicationID]
		if !ok || appID == "" {
			continue
		}
		active[ctr.ID] = struct{}{}

		c.mu.Lock()
		_, watching := c.watchers[ctr.ID]
		atCap := !watching && len(c.watchers) >= c.maxWatch
		var watchCtx context.Context
		var cancel context.CancelFunc
		if !watching && !atCap {
			watchCtx, cancel = context.WithCancel(ctx)
			c.watchers[ctr.ID] = cancel
		}
		c.mu.Unlock()

		if atCap {
			slog.Warn("log collector: watcher cap reached, skipping container",
				"container", ctr.Name, "cap", c.maxWatch)
			continue
		}
		if !watching {
			ctrCopy := ctr
			go func() {
				defer func() {
					c.mu.Lock()
					delete(c.watchers, ctrCopy.ID)
					c.mu.Unlock()
				}()
				c.watchContainer(watchCtx, ctrCopy)
			}()
		}
	}

	// Cancel watchers for containers that are no longer running.
	c.mu.Lock()
	for id, cancel := range c.watchers {
		if _, ok := active[id]; !ok {
			cancel()
			delete(c.watchers, id)
		}
	}
	c.mu.Unlock()
}

type logLine struct {
	stream  string
	message string
}

// chanWriter demultiplexes a Docker log stream into lines and sends them to ch.
type chanWriter struct {
	stream string
	ch     chan<- logLine
	buf    []byte
}

func (w *chanWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		msg := strings.TrimRight(string(w.buf[:idx]), "\r")
		msg = stripTimestamp(msg)
		if msg != "" {
			select {
			case w.ch <- logLine{stream: w.stream, message: msg}:
			default: // drop if channel full rather than blocking
			}
		}
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
}

// stripTimestamp removes the Docker-prepended RFC3339Nano timestamp prefix.
// Format: "2025-01-01T00:00:00.000000000Z message"
func stripTimestamp(line string) string {
	if len(line) < 20 {
		return line
	}
	idx := strings.IndexByte(line, ' ')
	if idx > 0 && idx <= 36 && line[4] == '-' && strings.ContainsAny(line[:idx], "TZ") {
		return line[idx+1:]
	}
	return line
}

func (c *Collector) watchContainer(ctx context.Context, ctr runtime.ContainerInfo) {
	appIDStr := ctr.Labels[labelApplicationID]
	var appUUID pgtype.UUID
	if err := appUUID.Scan(appIDStr); err != nil {
		slog.Warn("log collector: invalid application-id label", "container", ctr.Name, "app_id", appIDStr)
		return
	}

	// Resume from the last recorded log for this application so a restarted
	// API server (or a brand-new container) doesn't drop early log lines.
	// Zero time = fetch from the beginning of the container's log buffer.
	var since time.Time
	latest, err := c.queries.GetLatestApplicationLogTime(ctx, appUUID)
	if err == nil && latest.Valid {
		// Add 1ns so we don't re-ingest the exact line we already stored.
		since = latest.Time.Add(time.Nanosecond)
	}

	slog.Info("log collector: attaching to container",
		"container", ctr.Name,
		"app_id", appIDStr,
		"since", since.Format(time.RFC3339Nano))

	rc, err := c.runtime.ContainerLogsSince(ctx, ctr.ID, since)
	if err != nil {
		slog.Warn("log collector: failed to attach to container logs", "container", ctr.Name, "error", err)
		return
	}
	defer rc.Close()

	lines := make(chan logLine, 256)
	go func() {
		defer close(lines)
		stdout := &chanWriter{stream: "stdout", ch: lines}
		stderr := &chanWriter{stream: "stderr", ch: lines}
		stdcopy.StdCopy(stdout, stderr, rc) //nolint:errcheck
	}()

	var batch []generated.InsertApplicationLogParams
	flushTicker := time.NewTicker(c.flushEvery)
	defer flushTicker.Stop()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		c.flush(flushCtx, appUUID, appIDStr, batch)
		batch = batch[:0]
	}

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				// Container log stream ended (container stopped).
				flush(ctx)
				slog.Info("log collector: container log stream ended", "container", ctr.Name)
				return
			}
			batch = append(batch, generated.InsertApplicationLogParams{
				ApplicationID: appUUID,
				Stream:        line.stream,
				Message:       line.message,
			})
			if len(batch) >= c.batchSize {
				flush(ctx)
			}
		case <-flushTicker.C:
			flush(ctx)
		case <-ctx.Done():
			// rc.Close() via defer will unblock the stdcopy goroutine.
			// Drain remaining buffered lines before returning.
			drainTimeout := time.After(1 * time.Second)
		drainLoop:
			for {
				select {
				case line, ok := <-lines:
					if !ok {
						break drainLoop
					}
					batch = append(batch, generated.InsertApplicationLogParams{
						ApplicationID: appUUID,
						Stream:        line.stream,
						Message:       line.message,
					})
				case <-drainTimeout:
					break drainLoop
				}
			}
			// Use a bounded shutdown context so the final flush cannot hang
			// indefinitely on a slow DB while the collector is exiting.
			flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			flush(flushCtx)
			cancel()
			return
		}
	}
}

func (c *Collector) flush(ctx context.Context, appID pgtype.UUID, appIDStr string, batch []generated.InsertApplicationLogParams) {
	for _, p := range batch {
		if err := c.queries.InsertApplicationLog(ctx, p); err != nil {
			slog.Warn("log collector: failed to insert application log", "error", err)
			continue
		}

		// Publish to Redis for live WebSocket consumers.
		payload, _ := json.Marshal(map[string]any{
			"stream":  p.Stream,
			"message": p.Message,
		})
		appIDFormatted := fmt.Sprintf("%x-%x-%x-%x-%x",
			appID.Bytes[0:4], appID.Bytes[4:6],
			appID.Bytes[6:8], appID.Bytes[8:10],
			appID.Bytes[10:16])
		c.rdb.Publish(ctx, "app-logs:"+appIDFormatted, string(payload))
	}
}
