package handler

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/weiliang79/belune/internal/store/generated"
)

// The application detail page needs to answer "is what is running still what I
// saved?". These two helpers are the write half of that: every handler that
// changes something the running container was built or started with stamps one
// of them, and the deploy worker clears them again on success
// (see worker.clearChangeMarkers).
//
// Pick by asking what it takes to apply the change:
//
//   - markConfigChanged  a reload is enough — the container is recreated from
//     the image it is already running. Env vars, volumes, file mounts,
//     CPU/memory limits, runtime profile, health-check path.
//
//   - markSourceChanged  a real build or pull is required. source_image,
//     dockerfile_path, build_type_override, builder_image, branch, git
//     credentials.
//
// Source is the stronger of the two and is reported on its own, so a source
// change does not also need markConfigChanged — stamping both would leave a
// stale "Reload to apply" behind once the deploy cleared only the source
// marker.
//
// Both are best-effort. A failure here means the indicator is stale, which is
// not worth failing the user's save over — the save itself already succeeded.

func (h *Handler) markConfigChanged(ctx context.Context, applicationID pgtype.UUID) {
	if err := h.queries.TouchApplicationConfigChanged(ctx, applicationID); err != nil {
		slog.Warn("could not mark application config changed", "error", err, "application_id", uuidToString(applicationID))
	}
}

func (h *Handler) markSourceChanged(ctx context.Context, applicationID pgtype.UUID) {
	if err := h.queries.TouchApplicationSourceChanged(ctx, applicationID); err != nil {
		slog.Warn("could not mark application source changed", "error", err, "application_id", uuidToString(applicationID))
	}
}

// markApplicationUpdate handles the one write path that spans both categories.
// It diffs before against after rather than inspecting the request, because the
// service can override what was asked for — a preview child keeps its own
// branch no matter what the request said — and stamping a marker for a field
// that did not actually move would show an indicator the user cannot clear by
// doing what it asks.
//
// A no-op save (open the form, hit Save, change nothing) therefore stamps
// nothing, which is the behaviour that keeps the indicator trustworthy.
func (h *Handler) markApplicationUpdate(ctx context.Context, before, after generated.Application) {
	sourceChanged := before.SourceRepo != after.SourceRepo ||
		before.SourceImage != after.SourceImage ||
		before.DockerfilePath != after.DockerfilePath ||
		before.BuildTypeOverride != after.BuildTypeOverride ||
		before.BuilderImage != after.BuilderImage ||
		before.Branch != after.Branch ||
		before.GitIntegrationID != after.GitIntegrationID ||
		!bytes.Equal(before.GitCredentialsEncrypted, after.GitCredentialsEncrypted)

	// Not auto_deploy_branch: it only filters which pushes trigger a deploy, so
	// changing it alone needs no deploy to take effect. It moves in lockstep
	// with branch anyway, which is covered above.
	configChanged := before.CpuLimit != after.CpuLimit ||
		before.MemoryLimit != after.MemoryLimit ||
		before.HealthCheckPath != after.HealthCheckPath

	switch {
	case sourceChanged:
		h.markSourceChanged(ctx, after.ID)
	case configChanged:
		h.markConfigChanged(ctx, after.ID)
	}
}
