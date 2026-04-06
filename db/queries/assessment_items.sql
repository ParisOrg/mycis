-- name: ListAssessmentItems :many
SELECT
  ai.id,
  ai.assessment_id,
  ai.framework_item_id,
  ai.control_record_id,
  cr.owner_user_id,
  cr.reviewer_user_id,
  ai.status,
  ai.score,
  ai.priority,
  ai.due_date,
  cr.notes,
  ai.blocked_reason,
  ai.validated_at,
  ai.validated_by,
  ai.updated_by,
  ai.created_at,
  ai.updated_at,
  fg.code AS group_code,
  fg.title AS group_title,
  fi.code AS item_code,
  fi.title AS item_title,
  fi.summary AS item_summary,
  fi.tags,
  ou.name AS owner_name,
  ru.name AS reviewer_name,
  (ai.due_date < CURRENT_DATE AND ai.status NOT IN ('validated', 'not_applicable')) AS is_overdue
FROM assessment_items ai
JOIN control_records cr ON cr.id = ai.control_record_id
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN users ou ON ou.id = cr.owner_user_id
LEFT JOIN users ru ON ru.id = cr.reviewer_user_id
WHERE ai.assessment_id = $1
  AND (sqlc.narg(group_code)::text IS NULL OR fg.code = sqlc.narg(group_code)::text)
  AND (sqlc.narg(tag)::text IS NULL OR sqlc.narg(tag)::text = ANY(fi.tags))
  AND (sqlc.narg(status)::text IS NULL OR ai.status::text = sqlc.narg(status)::text)
  AND (sqlc.narg(owner_user_id)::text IS NULL OR cr.owner_user_id::text = sqlc.narg(owner_user_id)::text)
  AND (sqlc.narg(reviewer_user_id)::text IS NULL OR cr.reviewer_user_id::text = sqlc.narg(reviewer_user_id)::text)
  AND (sqlc.narg(overdue)::boolean IS NULL OR (
    sqlc.narg(overdue)::boolean = TRUE
    AND ai.due_date < CURRENT_DATE
    AND ai.status NOT IN ('validated', 'not_applicable')
  ))
ORDER BY
  CASE WHEN fi.code ~ '^[0-9]+(\.[0-9]+)?$' THEN 0 ELSE 1 END,
  CASE
    WHEN fi.code ~ '^[0-9]+(\.[0-9]+)?$' THEN split_part(fi.code, '.', 1)::int
  END NULLS LAST,
  CASE
    WHEN fi.code ~ '^[0-9]+(\.[0-9]+)?$' AND position('.' IN fi.code) > 0 THEN split_part(fi.code, '.', 2)::numeric
  END NULLS LAST,
  fi.code;

-- name: GetAssessmentItemDetail :one
SELECT
  ai.id,
  ai.assessment_id,
  ai.framework_item_id,
  ai.control_record_id,
  cr.owner_user_id,
  cr.reviewer_user_id,
  ai.status,
  ai.score,
  ai.priority,
  ai.due_date,
  cr.notes,
  cr.is_not_applicable,
  ai.blocked_reason,
  ai.validated_at,
  ai.validated_by,
  ai.updated_by,
  ai.created_at,
  ai.updated_at,
  a.name AS assessment_name,
  a.scope AS assessment_scope,
  f.label AS framework_label,
  f.version AS framework_version,
  fg.code AS group_code,
  fg.title AS group_title,
  fi.code AS item_code,
  fi.title AS item_title,
  fi.summary AS item_summary,
  fi.description AS item_description,
  fi.asset_class,
  fi.security_function,
  fi.tags,
  ou.name AS owner_name,
  ru.name AS reviewer_name,
  vu.name AS validated_by_name,
  (ai.due_date < CURRENT_DATE AND ai.status NOT IN ('validated', 'not_applicable')) AS is_overdue
FROM assessment_items ai
JOIN control_records cr ON cr.id = ai.control_record_id
JOIN assessments a ON a.id = ai.assessment_id
JOIN frameworks f ON f.id = a.framework_id
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN users ou ON ou.id = cr.owner_user_id
LEFT JOIN users ru ON ru.id = cr.reviewer_user_id
LEFT JOIN users vu ON vu.id = ai.validated_by
WHERE ai.id = $1;

-- name: UpdateAssessmentItem :one
UPDATE assessment_items
SET status = $2,
    score = sqlc.narg(score),
    priority = $3,
    due_date = $4,
    blocked_reason = sqlc.narg(blocked_reason),
    validated_at = sqlc.narg(validated_at),
    validated_by = sqlc.narg(validated_by),
    updated_by = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: BulkSetDueDate :exec
UPDATE assessment_items
SET due_date = $3,
    updated_by = $4,
    updated_at = NOW()
WHERE assessment_id = $1
  AND id = ANY($2::uuid[]);

-- name: BulkSetPriority :exec
UPDATE assessment_items
SET priority = $3,
    updated_by = $4,
    updated_at = NOW()
WHERE assessment_id = $1
  AND id = ANY($2::uuid[]);
