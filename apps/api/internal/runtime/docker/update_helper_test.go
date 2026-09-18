package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weiliang79/belune/internal/runtime"
)

// TestUpdateHelperLabels_NeutralisesComposeMembership pins the one property the
// self-updater's survival depends on: Compose must not consider the helper part
// of any service.
//
// ⚠️ Why this needs a test rather than a comment. A container inherits its
// IMAGE's labels, and an image built by `docker compose build` carries
// com.docker.compose.project/service — this repo's own devcontainer image does,
// with project=infra, service=api. The helper reuses Belune's image, and
// update.sh runs `docker compose up -d`, which may remove the excess containers
// of a service it is recreating. That would kill the updater mid-update, right
// after it has rewritten .env.
//
// The published images happen not to carry those labels, so without the
// explicit override this holds by ABSENCE — it would break silently the day
// someone rebuilds with compose, and nothing would fail until a real update
// destroyed itself on a real install.
func TestUpdateHelperLabels_NeutralisesComposeMembership(t *testing.T) {
	labels := updateHelperLabels()

	// Compose matches a service by project AND service name. An empty value
	// overrides the image's (verified against a real image that carries them),
	// and cannot match a real project or service.
	for _, k := range []string{composeProjectLabel, composeServiceLabel} {
		v, ok := labels[k]
		assert.True(t, ok, "%s must be set explicitly, not left to inherit from the image", k)
		assert.Empty(t, v, "%s must be blanked so Compose cannot claim this container", k)
	}

	// Still discoverable as ours: the reaper spares running helpers by
	// LabelHelper, and the 409 conflict check finds it by LabelUpdateHelper.
	assert.Equal(t, labelValue, labels[labelManagedBy])
	assert.Equal(t, "true", labels[runtime.LabelHelper])
	assert.Equal(t, "true", labels[runtime.LabelUpdateHelper])
}
