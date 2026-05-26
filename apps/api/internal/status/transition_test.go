package status_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ungweiliang/selfhost-paas/internal/status"
)

func TestValidTransition(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		// Allowed forward progressions
		{status.DeploymentPending, status.DeploymentBuilding, true},
		{status.DeploymentPending, status.DeploymentDeploying, true},
		{status.DeploymentBuilding, status.DeploymentDeploying, true},
		{status.DeploymentDeploying, status.DeploymentSuccess, true},

		// Any state → failed is always permitted (error catch-all)
		{status.DeploymentPending, status.DeploymentFailed, true},
		{status.DeploymentBuilding, status.DeploymentFailed, true},
		{status.DeploymentDeploying, status.DeploymentFailed, true},
		{status.DeploymentSuccess, status.DeploymentFailed, true},
		{status.DeploymentFailed, status.DeploymentFailed, true},

		// Terminal states must not advance further
		{status.DeploymentSuccess, status.DeploymentDeploying, false},
		{status.DeploymentSuccess, status.DeploymentBuilding, false},
		{status.DeploymentSuccess, status.DeploymentPending, false},
		{status.DeploymentFailed, status.DeploymentSuccess, false},
		{status.DeploymentFailed, status.DeploymentDeploying, false},
		{status.DeploymentFailed, status.DeploymentBuilding, false},

		// No backwards transitions
		{status.DeploymentDeploying, status.DeploymentBuilding, false},
		{status.DeploymentDeploying, status.DeploymentPending, false},
		{status.DeploymentBuilding, status.DeploymentPending, false},

		// Unknown from-state
		{"unknown", status.DeploymentBuilding, false},
		{"", status.DeploymentBuilding, false},
	}

	for _, tc := range tests {
		t.Run(tc.from+"→"+tc.to, func(t *testing.T) {
			got := status.ValidTransition(tc.from, tc.to)
			assert.Equal(t, tc.want, got)
		})
	}
}
