-- name: CreateAuditLog :exec
INSERT INTO audit_log (
  entity_type,
  entity_id,
  action,
  actor_user_id,
  payload_json
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5
);

-- name: ListAuditLogByEntity :many
SELECT
  a.*,
  u.name AS actor_name
FROM audit_log a
JOIN users u ON u.id = a.actor_user_id
WHERE a.entity_type = $1
  AND a.entity_id = $2
ORDER BY a.created_at DESC;

-- name: CreateComment :one
INSERT INTO comments (
  control_record_id,
  user_id,
  body
) VALUES (
  $1,
  $2,
  $3
)
RETURNING *;

-- name: ListCommentsByControlRecord :many
SELECT
  c.*,
  u.name AS user_name
FROM comments c
JOIN users u ON u.id = c.user_id
WHERE c.control_record_id = $1
ORDER BY c.created_at DESC;

-- name: CreateEvidenceLink :one
INSERT INTO evidence_links (
  control_record_id,
  label,
  url,
  created_by
) VALUES (
  $1,
  $2,
  $3,
  $4
)
RETURNING *;

-- name: ListEvidenceLinksByControlRecord :many
SELECT
  e.*,
  u.name AS created_by_name
FROM evidence_links e
JOIN users u ON u.id = e.created_by
WHERE e.control_record_id = $1
ORDER BY e.created_at DESC;
