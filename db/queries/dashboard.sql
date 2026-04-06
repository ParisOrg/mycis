-- name: GetDashboardOverview :one
SELECT
  COUNT(*)::int AS total_items,
  COUNT(*) FILTER (WHERE status IN ('validated', 'not_applicable'))::int AS completed_items,
  COUNT(*) FILTER (WHERE status = 'ready_for_review')::int AS ready_for_review_items,
  COUNT(*) FILTER (WHERE status = 'blocked')::int AS blocked_items,
  COUNT(*) FILTER (
    WHERE due_date < CURRENT_DATE
      AND status NOT IN ('validated', 'not_applicable')
  )::int AS overdue_items,
  COALESCE(ROUND(AVG(score)::numeric, 1), 0)::numeric AS average_score
FROM assessment_items
WHERE assessment_id = $1;

-- name: ListDashboardGroupCompletion :many
SELECT
  fg.code AS group_code,
  fg.title AS group_title,
  COUNT(ai.id)::int AS total_items,
  COUNT(ai.id) FILTER (WHERE ai.status IN ('validated', 'not_applicable'))::int AS completed_items,
  COALESCE(ROUND(AVG(ai.score)::numeric, 1), 0)::numeric AS average_score
FROM assessment_items ai
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
WHERE ai.assessment_id = $1
GROUP BY fg.id
ORDER BY
  CASE WHEN fg.code ~ '^[0-9]+$' THEN 0 ELSE 1 END,
  CASE WHEN fg.code ~ '^[0-9]+$' THEN fg.code::int END NULLS LAST,
  fg.code;

-- name: ListDashboardOwnerWorkload :many
SELECT
  u.id,
  u.name,
  COUNT(ai.id)::int AS total_items,
  COUNT(ai.id) FILTER (WHERE ai.status IN ('validated', 'not_applicable'))::int AS completed_items,
  COUNT(ai.id) FILTER (
    WHERE ai.due_date < CURRENT_DATE
      AND ai.status NOT IN ('validated', 'not_applicable')
  )::int AS overdue_items
FROM users u
JOIN control_records cr ON cr.owner_user_id = u.id
JOIN assessment_items ai ON ai.control_record_id = cr.id
WHERE ai.assessment_id = $1
GROUP BY u.id
ORDER BY total_items DESC, u.name ASC;

-- name: ListDashboardOverdueItems :many
SELECT
  ai.id,
  fi.code AS item_code,
  fi.title AS item_title,
  fg.code AS group_code,
  COALESCE(ou.name, 'Unassigned') AS owner_name,
  ai.due_date
FROM assessment_items ai
JOIN control_records cr ON cr.id = ai.control_record_id
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN users ou ON ou.id = cr.owner_user_id
WHERE ai.assessment_id = $1
  AND ai.due_date < CURRENT_DATE
  AND ai.status NOT IN ('validated', 'not_applicable')
ORDER BY ai.due_date ASC, fi.code ASC
LIMIT 10;

-- name: ListDashboardReviewQueue :many
SELECT
  ai.id,
  fi.code AS item_code,
  fi.title AS item_title,
  fg.code AS group_code,
  COALESCE(ou.name, 'Unassigned') AS owner_name,
  COALESCE(ru.name, 'Unassigned') AS reviewer_name,
  ai.updated_at
FROM assessment_items ai
JOIN control_records cr ON cr.id = ai.control_record_id
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN users ou ON ou.id = cr.owner_user_id
LEFT JOIN users ru ON ru.id = cr.reviewer_user_id
WHERE ai.assessment_id = $1
  AND ai.status = 'ready_for_review'
ORDER BY ai.updated_at DESC
LIMIT 10;

-- name: ListDashboardLowScoreItems :many
SELECT
  ai.id,
  fi.code AS item_code,
  fi.title AS item_title,
  fg.code AS group_code,
  COALESCE(ou.name, 'Unassigned') AS owner_name,
  ai.score
FROM assessment_items ai
JOIN control_records cr ON cr.id = ai.control_record_id
JOIN framework_items fi ON fi.id = ai.framework_item_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN users ou ON ou.id = cr.owner_user_id
WHERE ai.assessment_id = $1
  AND ai.score IS NOT NULL
  AND ai.score <= 2
ORDER BY ai.score ASC, fi.code ASC
LIMIT 10;
