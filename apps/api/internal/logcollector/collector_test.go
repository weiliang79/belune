package logcollector_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ungweiliang/selfhost-paas/internal/logcollector"
	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
	"github.com/ungweiliang/selfhost-paas/internal/testutil"
)

var (
	testPool    *pgxpool.Pool
	testQueries *generated.Queries
)

func TestMain(m *testing.M) {
	pool, queries, teardown := testutil.SetupTestDB()
	testPool = pool
	testQueries = queries
	code := m.Run()
	teardown()
	os.Exit(code)
}

// startMiniredis returns a go-redis Client backed by an in-process server.
func startMiniredis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// makeDockerStdoutStream creates a Docker multiplexed log stream (stdout frames)
// from the given text lines. stdcopy.StdCopy understands this format.
// Frame layout: [stream_type(1B), 0, 0, 0, size_big_endian(4B), data...]
func makeDockerStdoutStream(lines ...string) io.ReadCloser {
	var buf bytes.Buffer
	for _, line := range lines {
		data := []byte(line + "\n")
		var hdr [8]byte
		hdr[0] = 1 // stdout
		binary.BigEndian.PutUint32(hdr[4:], uint32(len(data)))
		buf.Write(hdr[:])
		buf.Write(data)
	}
	return io.NopCloser(&buf)
}

// formatUUID formats a pgtype.UUID as a lowercase hyphenated string.
func formatUUID(u pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}

// logStreamRuntime embeds MockContainerRuntime and overrides ContainerLogsSince
// so tests can inject a fake log stream and capture the `since` argument.
type logStreamRuntime struct {
	*testutil.MockContainerRuntime
	mu         sync.Mutex
	sinceCalls []time.Time
	streamOnce sync.Once
	stream     io.ReadCloser
}

func (r *logStreamRuntime) ContainerLogsSince(_ context.Context, _ string, since time.Time) (io.ReadCloser, error) {
	r.mu.Lock()
	r.sinceCalls = append(r.sinceCalls, since)
	r.mu.Unlock()
	// Return the real stream on the first call; return empty on subsequent calls
	// so a re-triggered watcher (after the container reappears in sync) does not
	// block waiting for data that will never arrive.
	var rc io.ReadCloser
	r.streamOnce.Do(func() { rc = r.stream })
	if rc == nil {
		rc = io.NopCloser(bytes.NewReader(nil))
	}
	return rc, nil
}

// seedCollectorApp creates a minimal user→project→application in the DB and
// returns the application so its UUID can be used as the container label.
func seedCollectorApp(t *testing.T) generated.Application {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	user, err := testQueries.CreateUser(ctx, generated.CreateUserParams{
		Email:        "col-" + suffix + "@test.com",
		PasswordHash: "x",
		Role:         "admin",
	})
	require.NoError(t, err)

	project, err := testQueries.CreateProject(ctx, generated.CreateProjectParams{
		Name:   "Collector Project",
		Slug:   "col-" + suffix,
		UserID: user.ID,
	})
	require.NoError(t, err)

	app, err := testQueries.CreateApplication(ctx, generated.CreateApplicationParams{
		ProjectID: project.ID,
		Name:      "Collector App",
		Slug:      "col-app-" + suffix,
		Type:      "image",
		BuildType: "image",
	})
	require.NoError(t, err)
	return app
}

// TestCollector_Run_ExitsOnCtxCancel is the Phase 1 ctx-cancel regression test.
// It verifies that Run returns promptly when its context is cancelled, confirming
// no goroutine is leaked after the collector shuts down.
func TestCollector_Run_ExitsOnCtxCancel(t *testing.T) {
	// Empty container list → no watchers started → no DB/Redis access during run.
	rt := &logStreamRuntime{
		MockContainerRuntime: &testutil.MockContainerRuntime{},
	}
	rdb := startMiniredis(t)
	c := logcollector.New(rt, testQueries, rdb)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx)
	}()

	// Empty container list → Run just waits on ctx.Done; cancel immediately.
	cancel()

	select {
	case <-done:
		// Run exited cleanly — no goroutine leak.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit within 2s after ctx cancellation — goroutine leak suspected")
	}
}

// TestCollector_IngestsContainerLogs verifies the happy-path collection cycle:
// a running container is discovered, its log stream is consumed, and the log
// lines appear in the application_logs table.
func TestCollector_IngestsContainerLogs(t *testing.T) {
	t.Cleanup(func() { _ = testutil.TruncateAll(context.Background(), testPool) })

	app := seedCollectorApp(t)
	appIDStr := formatUUID(app.ID)

	rt := &logStreamRuntime{
		MockContainerRuntime: &testutil.MockContainerRuntime{
			ListContainers_: []runtime.ContainerInfo{
				{
					ID:     "ctr-abc123",
					Name:   "test-container",
					Status: "running",
					Labels: map[string]string{"application-id": appIDStr},
				},
			},
		},
		stream: makeDockerStdoutStream("hello from collector", "second line"),
	}
	rdb := startMiniredis(t)
	c := logcollector.New(rt, testQueries, rdb)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	// Poll until the two log lines land in the DB (the stream is finite so the
	// watcher flushes immediately after EOF).
	assert.Eventually(t, func() bool {
		logs, err := testQueries.ListApplicationLogsByApplication(context.Background(),
			generated.ListApplicationLogsByApplicationParams{
				ApplicationID: app.ID,
				Limit:         10,
				Offset:        0,
			})
		return err == nil && len(logs) >= 2
	}, 5*time.Second, 50*time.Millisecond, "expected 2 log lines in DB within 5s")

	logs, err := testQueries.ListApplicationLogsByApplication(context.Background(),
		generated.ListApplicationLogsByApplicationParams{
			ApplicationID: app.ID,
			Limit:         10,
			Offset:        0,
		})
	require.NoError(t, err)
	require.Len(t, logs, 2)

	messages := []string{logs[0].Message, logs[1].Message}
	assert.Contains(t, messages, "hello from collector")
	assert.Contains(t, messages, "second line")
	assert.Equal(t, "stdout", logs[0].Stream)
}

// TestCollector_ResumesFromLastLogTime verifies the retention boundary: when the
// collector attaches to a container whose application already has stored logs, it
// passes since=lastRecordedAt+1ns to ContainerLogsSince so no lines are re-ingested.
func TestCollector_ResumesFromLastLogTime(t *testing.T) {
	t.Cleanup(func() { _ = testutil.TruncateAll(context.Background(), testPool) })

	app := seedCollectorApp(t)
	appIDStr := formatUUID(app.ID)
	ctx := context.Background()

	// Pre-insert a log line so there is a recorded_at to resume from.
	require.NoError(t, testQueries.InsertApplicationLog(ctx, generated.InsertApplicationLogParams{
		ApplicationID: app.ID,
		Stream:        "stdout",
		Message:       "pre-existing log",
	}))

	// Fetch the exact timestamp the DB stamped; the collector must resume from this + 1ns.
	latest, err := testQueries.GetLatestApplicationLogTime(ctx, app.ID)
	require.NoError(t, err)
	require.True(t, latest.Valid, "GetLatestApplicationLogTime must return a valid time after insert")
	expectedSince := latest.Time.Add(time.Nanosecond)

	rt := &logStreamRuntime{
		MockContainerRuntime: &testutil.MockContainerRuntime{
			ListContainers_: []runtime.ContainerInfo{
				{
					ID:     "ctr-resume",
					Name:   "resume-container",
					Status: "running",
					Labels: map[string]string{"application-id": appIDStr},
				},
			},
		},
		stream: io.NopCloser(bytes.NewReader(nil)), // no new lines — we only check `since`
	}
	rdb := startMiniredis(t)
	c := logcollector.New(rt, testQueries, rdb)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(cancelCtx)

	// Wait until ContainerLogsSince has been called at least once.
	assert.Eventually(t, func() bool {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		return len(rt.sinceCalls) >= 1
	}, 3*time.Second, 20*time.Millisecond, "ContainerLogsSince not called within 3s")

	rt.mu.Lock()
	gotSince := rt.sinceCalls[0]
	rt.mu.Unlock()

	// Postgres timestamps have microsecond precision; Go's Add(time.Nanosecond)
	// produces a sub-microsecond offset, but the comparison should still be exact
	// because both the test and the collector derive from the same DB value.
	assert.Equal(t, expectedSince, gotSince,
		"collector must resume from lastRecordedAt+1ns, not from zero time")
}
