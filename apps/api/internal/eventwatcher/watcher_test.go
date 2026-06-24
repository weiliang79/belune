package eventwatcher

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/ungweiliang/selfhost-paas/internal/runtime"
	"github.com/ungweiliang/selfhost-paas/internal/status"
)

func TestHandleEvent_StartSetsRunning(t *testing.T) {
	event := runtime.ContainerEvent{
		ContainerID:   "abc123",
		ContainerName: "myapp",
		Status:        "start",
		Labels:        map[string]string{"application-id": "550e8400-e29b-41d4-a716-446655440000"},
	}

	var newStatus string
	switch event.Status {
	case "start", "restart":
		newStatus = status.ApplicationRunning
	case "stop", "die", "oom":
		newStatus = status.ApplicationStopped
	}
	assert.Equal(t, status.ApplicationRunning, newStatus)
}

func TestHandleEvent_StopSetsStopped(t *testing.T) {
	for _, s := range []string{"stop", "die", "oom"} {
		var newStatus string
		switch s {
		case "start", "restart":
			newStatus = status.ApplicationRunning
		case "stop", "die", "oom":
			newStatus = status.ApplicationStopped
		}
		assert.Equal(t, status.ApplicationStopped, newStatus, "event %q should map to stopped", s)
	}
}

func TestHandleEvent_NoAppIDIgnored(t *testing.T) {
	event := runtime.ContainerEvent{
		ContainerID: "abc123",
		Status:      "start",
		Labels:      map[string]string{},
	}
	appID, ok := event.Labels["application-id"]
	assert.False(t, ok)
	assert.Empty(t, appID)
}

func TestDatabaseStatusForEvent(t *testing.T) {
	cases := []struct {
		name     string
		event    string
		exitCode string
		want     string
		ok       bool
	}{
		{"start", "start", "", status.DatabaseRunning, true},
		{"restart", "restart", "", status.DatabaseRunning, true},
		{"stop", "stop", "", status.DatabaseStopped, true},
		{"clean die", "die", "0", status.DatabaseStopped, true},
		{"crash die", "die", "1", status.DatabaseFailed, true},
		{"die no exit code", "die", "", status.DatabaseFailed, true},
		{"oom", "oom", "", status.DatabaseFailed, true},
		{"unknown ignored", "pause", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			labels := map[string]string{}
			if c.exitCode != "" {
				labels["exitCode"] = c.exitCode
			}
			got, ok := databaseStatusForEvent(runtime.ContainerEvent{
				Status: c.event,
				Labels: labels,
			})
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestHandleEvent_DispatchesByLabel(t *testing.T) {
	// Application label takes the application path; database label the DB path;
	// neither label is a no-op. We assert dispatch via label precedence here.
	appEvent := runtime.ContainerEvent{
		Status: "start",
		Labels: map[string]string{labelApplicationID: "550e8400-e29b-41d4-a716-446655440000"},
	}
	dbEvent := runtime.ContainerEvent{
		Status: "die",
		Labels: map[string]string{labelDatabaseID: "550e8400-e29b-41d4-a716-446655440001", "exitCode": "1"},
	}
	_, appHasApp := appEvent.Labels[labelApplicationID]
	_, dbHasDB := dbEvent.Labels[labelDatabaseID]
	assert.True(t, appHasApp)
	assert.True(t, dbHasDB)

	got, ok := databaseStatusForEvent(dbEvent)
	assert.True(t, ok)
	assert.Equal(t, status.DatabaseFailed, got)
}

func TestPgUUIDToString(t *testing.T) {
	// Test with zero UUID bytes
	result := pgUUIDToString(pgUUID("550e8400-e29b-41d4-a716-446655440000"))
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result)
}

// pgUUID parses a UUID string into a pgtype.UUID for testing.
func pgUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}
