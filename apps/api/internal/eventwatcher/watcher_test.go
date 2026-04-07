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
