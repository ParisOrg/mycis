-- name: CreateFramework :one
INSERT INTO frameworks (
  slug,
  label,
  version,
  status
) VALUES (
  $1,
  $2,
  $3,
  $4
)
RETURNING *;

-- name: GetFrameworkBySlugVersion :one
SELECT *
FROM frameworks
WHERE slug = $1
  AND version = $2;

-- name: GetFrameworkByID :one
SELECT *
FROM frameworks
WHERE id = $1;

-- name: CreateFrameworkGroup :one
INSERT INTO framework_groups (
  framework_id,
  code,
  title,
  summary,
  description,
  sort_order
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
)
RETURNING *;

-- name: UpsertFrameworkGroup :one
INSERT INTO framework_groups (
  framework_id,
  code,
  title,
  summary,
  description,
  sort_order
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
)
ON CONFLICT (framework_id, code) DO UPDATE
  SET title       = EXCLUDED.title,
      summary     = EXCLUDED.summary,
      description = EXCLUDED.description,
      sort_order  = EXCLUDED.sort_order,
      is_active   = TRUE
RETURNING *;

-- name: CreateFrameworkItem :one
INSERT INTO framework_items (
  framework_id,
  framework_group_id,
  code,
  title,
  summary,
  description,
  sort_order,
  asset_class,
  security_function,
  tags
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10
)
RETURNING *;

-- name: UpsertFrameworkItem :one
INSERT INTO framework_items (
  framework_id,
  framework_group_id,
  code,
  title,
  summary,
  description,
  sort_order,
  asset_class,
  security_function,
  tags
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10
)
ON CONFLICT (framework_id, code) DO UPDATE
  SET framework_group_id  = EXCLUDED.framework_group_id,
      title               = EXCLUDED.title,
      summary             = EXCLUDED.summary,
      description         = EXCLUDED.description,
      sort_order          = EXCLUDED.sort_order,
      asset_class         = EXCLUDED.asset_class,
      security_function   = EXCLUDED.security_function,
      tags                = EXCLUDED.tags,
      is_active           = TRUE
RETURNING *;

-- name: GetFrameworkItemByCode :one
SELECT *
FROM framework_items
WHERE framework_id = $1
  AND code = $2;

-- name: DeactivateMissingFrameworkGroups :exec
UPDATE framework_groups
SET is_active = FALSE
WHERE framework_id = sqlc.arg(framework_id)::uuid
  AND NOT (code = ANY(sqlc.arg(codes)::text[]));

-- name: DeactivateMissingFrameworkItems :exec
UPDATE framework_items
SET is_active = FALSE
WHERE framework_id = sqlc.arg(framework_id)::uuid
  AND NOT (code = ANY(sqlc.arg(codes)::text[]));

-- name: ListFrameworksWithCounts :many
SELECT
  f.*,
  COUNT(DISTINCT fg.id)::int AS group_count,
  COUNT(DISTINCT fi.id)::int AS item_count
FROM frameworks f
LEFT JOIN framework_groups fg ON fg.framework_id = f.id
  AND fg.is_active
LEFT JOIN framework_items fi ON fi.framework_id = f.id
  AND fi.is_active
GROUP BY f.id
ORDER BY f.created_at DESC;

-- name: DeleteFramework :exec
DELETE FROM frameworks WHERE id = $1;

-- name: ListFrameworkGroupsByFramework :many
SELECT
  fg.*,
  COUNT(fi.id)::int AS item_count
FROM framework_groups fg
LEFT JOIN framework_items fi ON fi.framework_group_id = fg.id
  AND fi.is_active
WHERE fg.framework_id = $1
  AND fg.is_active
GROUP BY fg.id
ORDER BY fg.sort_order;
