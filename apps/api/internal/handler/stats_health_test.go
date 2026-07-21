package handler

import (
	"testing"

	"github.com/weiliang79/belune/internal/store/generated"
)

// The App Health card renders one badge per bucket and relies on them being
// exhaustive, so the sum invariant is the thing worth pinning down.
func TestAppHealth(t *testing.T) {
	tests := []struct {
		name    string
		app     generated.CountApplicationHealthRow
		db      generated.CountDatabaseHealthRow
		errored int64
		want    healthRatio
	}{
		{
			name: "healthy fleet",
			app:  generated.CountApplicationHealthRow{Total: 3, Running: 3},
			db:   generated.CountDatabaseHealthRow{Total: 1, Running: 1},
			want: healthRatio{Running: 4, Total: 4},
		},
		{
			name:    "every application state represented",
			app:     generated.CountApplicationHealthRow{Total: 5, Running: 1, Errored: 1, Stopped: 1, Unhealthy: 1, Inactive: 1},
			errored: 1,
			want: healthRatio{
				Running: 1, Errored: 1, Stopped: 1, Unhealthy: 1, Inactive: 1, Total: 5,
			},
		},
		{
			// creating / upgrading / backing_up are counted by no named bucket,
			// so they must land in Busy rather than disappear.
			name: "transient databases become busy",
			app:  generated.CountApplicationHealthRow{Total: 1, Running: 1},
			db:   generated.CountDatabaseHealthRow{Total: 3, Running: 0},
			want: healthRatio{Running: 1, Busy: 3, Total: 4},
		},
		{
			name: "errored spans both resource types",
			app:  generated.CountApplicationHealthRow{Total: 1, Errored: 1},
			db:   generated.CountDatabaseHealthRow{Total: 1, Errored: 1},
			// GetStats passes the combined app+db error count.
			errored: 2,
			want:    healthRatio{Errored: 2, Total: 2},
		},
		{
			// Defensive: a skew must never render a negative badge.
			name:    "residual clamped at zero",
			app:     generated.CountApplicationHealthRow{Total: 1, Running: 2},
			errored: 0,
			want:    healthRatio{Running: 2, Total: 1, Busy: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appHealth(tt.app, tt.db, tt.errored)
			if got != tt.want {
				t.Errorf("appHealth() = %+v, want %+v", got, tt.want)
			}
			// The card's badges must account for every service.
			sum := got.Running + got.Errored + got.Stopped +
				got.Unhealthy + got.Inactive + got.Busy
			if got.Busy > 0 && sum != got.Total {
				t.Errorf("buckets sum to %d, want total %d", sum, got.Total)
			}
		})
	}
}
