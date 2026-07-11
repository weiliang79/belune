package logcollector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceOf(t *testing.T) {
	appID := "11111111-1111-1111-1111-111111111111"
	dbID := "22222222-2222-2222-2222-222222222222"

	tests := []struct {
		name     string
		labels   map[string]string
		wantOK   bool
		wantType string
		wantID   string
		wantName string
	}{
		{
			name:     "application container",
			labels:   map[string]string{labelApplicationID: appID},
			wantOK:   true,
			wantType: sourceApplication,
			wantID:   appID,
		},
		{
			name:     "database container",
			labels:   map[string]string{labelDatabaseID: dbID},
			wantOK:   true,
			wantType: sourceDatabase,
			wantID:   dbID,
		},
		{
			// Caddy has no UUID of its own, so it gets a fixed synthetic one:
			// container_logs keys every row by UUID, and adding a column just for
			// infrastructure containers would not pay for itself.
			name:     "caddy is a system source with a stable synthetic id",
			labels:   map[string]string{labelSystem: SystemCaddy},
			wantOK:   true,
			wantType: sourceSystem,
			wantID:   CaddySourceID,
			wantName: SystemCaddy,
		},
		{
			// An unrecognised system component would otherwise be inserted with an
			// empty source_id and fail the UUID scan.
			name:   "unknown system component is ignored",
			labels: map[string]string{labelSystem: "postgres"},
			wantOK: false,
		},
		{
			name:   "unlabelled container is not collected",
			labels: map[string]string{"com.docker.compose.project": "infra"},
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, ok := sourceOf(tc.labels)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.wantType, src.typ)
			assert.Equal(t, tc.wantID, src.id)
			assert.Equal(t, tc.wantName, src.name)
		})
	}
}

// The synthetic id must be a real UUID or every Caddy log line would fail the
// scan into container_logs.source_id.
func TestCaddySourceIDIsAUUID(t *testing.T) {
	assert.Len(t, CaddySourceID, 36)
	assert.Equal(t, "00000000-0000-0000-0000-0000000000ca", CaddySourceID)
}
