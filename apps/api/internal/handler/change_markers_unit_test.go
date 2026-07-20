package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store/generated"
)

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func TestPendingChange(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		app  generated.Application
		want string
	}{
		{
			name: "nothing changed",
			app:  generated.Application{LastDeployedAt: ts(now)},
			want: "",
		},
		{
			name: "config marker set",
			app:  generated.Application{LastDeployedAt: ts(now), ConfigChangedAt: ts(now)},
			want: "config",
		},
		{
			name: "source marker set",
			app:  generated.Application{LastDeployedAt: ts(now), SourceChangedAt: ts(now)},
			want: "source",
		},
		{
			// Source outranks config: a deploy applies both, so offering
			// "Reload to apply" here would name an action that cannot finish
			// the job.
			name: "both set reports the stronger one",
			app: generated.Application{
				LastDeployedAt:  ts(now),
				ConfigChangedAt: ts(now),
				SourceChangedAt: ts(now),
			},
			want: "source",
		},
		{
			// The false positive the suppression rule exists to prevent: an
			// app configured before its first deploy would otherwise read
			// "needs redeploy" from birth.
			name: "suppressed until the first successful deploy",
			app: generated.Application{
				ConfigChangedAt: ts(now),
				SourceChangedAt: ts(now),
			},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pendingChange(c.app); got != c.want {
				t.Errorf("pendingChange() = %q, want %q", got, c.want)
			}
		})
	}
}
