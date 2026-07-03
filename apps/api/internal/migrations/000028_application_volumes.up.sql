-- Per-application persistent volume mounts.
--
-- Applications otherwise run with a read-only rootfs + tmpfs only (see
-- deploy_task.go createAndStart) — i.e. fully stateless. A row here declares a
-- managed Docker named volume that the deploy worker mounts at mount_path on
-- every (re)deploy, so data survives redeploys. The Docker volume itself is
-- named deterministically from application_id + name (naming.AppVolumeName) and
-- carries a do-not-prune label so the cleanup worker cannot delete user data.
--
-- name is stored already-slugified (mirrors how application slugs are stored)
-- and doubles as the volume-name component, so it is unique per application.
-- mount_path is an absolute in-container path and is likewise unique per
-- application (cannot mount two volumes at the same path).
CREATE TABLE application_volumes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    mount_path     VARCHAR(500) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_application_volumes_app ON application_volumes(application_id);
CREATE UNIQUE INDEX idx_application_volumes_app_name ON application_volumes(application_id, name);
CREATE UNIQUE INDEX idx_application_volumes_app_mount ON application_volumes(application_id, mount_path);
