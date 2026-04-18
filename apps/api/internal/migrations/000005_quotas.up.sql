-- Per-user and per-project quotas.
-- Aggregate caps layered on top of per-container resource limits.
-- NULL on any cap column = unlimited for that dimension.
CREATE TABLE quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope VARCHAR(20) NOT NULL CHECK (scope IN ('user', 'project')),
    scope_id UUID NOT NULL,
    max_applications INTEGER,
    max_cpu DOUBLE PRECISION,
    max_memory_mb BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_quotas_scope_id ON quotas(scope, scope_id);
