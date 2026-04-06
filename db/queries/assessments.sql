-- name: CreateAssessment :one
INSERT INTO assessments (
  framework_id,
  name,
  scope,
  start_date,
  due_date,
  status,
  created_by
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7
)
RETURNING *;

-- name: CreateAssessmentItemsFromControlRecords :exec
INSERT INTO assessment_items (
  assessment_id,
  framework_item_id,
  control_record_id,
  status,
  priority,
  due_date,
  updated_by
)
SELECT
  sqlc.arg(assessment_id)::uuid,
  cr.framework_item_id,
  cr.id,
  'not_started'::assessment_item_status,
  'medium'::item_priority,
  sqlc.arg(due_date)::date,
  sqlc.arg(updated_by)::uuid
FROM control_records cr
WHERE cr.assessment_id = sqlc.arg(assessment_id)::uuid;

-- name: GetAssessmentByID :one
SELECT
  a.*,
  f.label AS framework_label,
  f.version AS framework_version
FROM assessments a
JOIN frameworks f ON f.id = a.framework_id
WHERE a.id = $1;

-- name: ListAssessments :many
SELECT
  a.id,
  a.framework_id,
  a.name,
  a.scope,
  a.start_date,
  a.due_date,
  a.status,
  a.created_by,
  a.created_at,
  a.updated_at,
  f.label AS framework_label,
  f.version AS framework_version,
  COUNT(ai.id)::int AS item_count,
  COUNT(ai.id) FILTER (WHERE ai.status IN ('validated', 'not_applicable'))::int AS completed_count
FROM assessments a
JOIN frameworks f ON f.id = a.framework_id
LEFT JOIN assessment_items ai ON ai.assessment_id = a.id
GROUP BY a.id, f.label, f.version
ORDER BY a.created_at DESC;
