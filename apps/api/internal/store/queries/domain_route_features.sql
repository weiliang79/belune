-- name: ListRouteFeaturesByDomain :many
SELECT * FROM domain_route_features
WHERE domain_id = $1
ORDER BY feature_type;

-- name: UpsertRouteFeature :one
INSERT INTO domain_route_features (domain_id, feature_type, config, enabled)
VALUES ($1, $2, $3, $4)
ON CONFLICT (domain_id, feature_type) DO UPDATE
SET config = EXCLUDED.config, enabled = EXCLUDED.enabled, updated_at = NOW()
RETURNING *;

-- name: DeleteRouteFeature :exec
DELETE FROM domain_route_features WHERE id = $1;

-- name: DeleteRouteFeaturesByDomain :exec
DELETE FROM domain_route_features WHERE domain_id = $1;
