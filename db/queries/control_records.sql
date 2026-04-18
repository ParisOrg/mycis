-- name: CreateControlRecordsForAssessment :exec
INSERT INTO control_records (assessment_id, framework_item_id)
SELECT sqlc.arg(assessment_id)::uuid, fi.id
FROM framework_items fi
JOIN framework_groups fg ON fg.id = fi.framework_group_id
WHERE fi.framework_id = sqlc.arg(framework_id)::uuid
  AND fi.is_active
  AND fg.is_active;

-- name: CopyControlRecordsFromPreviousAssessment :exec
INSERT INTO control_records (
  assessment_id,
  framework_item_id,
  owner_user_id,
  reviewer_user_id
)
SELECT
  sqlc.arg(assessment_id)::uuid,
  fi.id,
  cr.owner_user_id,
  cr.reviewer_user_id
FROM assessments a
JOIN framework_items fi ON fi.framework_id = a.framework_id
JOIN framework_groups fg ON fg.id = fi.framework_group_id
LEFT JOIN assessment_items ai ON ai.assessment_id = sqlc.arg(previous_assessment_id)::uuid
  AND ai.framework_item_id = fi.id
LEFT JOIN control_records cr ON cr.id = ai.control_record_id
WHERE a.id = sqlc.arg(previous_assessment_id)::uuid
  AND fi.is_active
  AND fg.is_active;

-- name: GetControlRecord :one
SELECT * FROM control_records WHERE id = $1;

-- name: GetControlRecordByAssessmentItem :one
SELECT cr.*
FROM control_records cr
JOIN assessment_items ai ON ai.control_record_id = cr.id
WHERE ai.id = $1;

-- name: UpdateControlRecordOwner :exec
UPDATE control_records
SET owner_user_id = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateControlRecordReviewer :exec
UPDATE control_records
SET reviewer_user_id = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateControlRecordNotes :exec
UPDATE control_records
SET notes = $2,
    is_not_applicable = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: BulkAssignControlRecordOwner :exec
UPDATE control_records
SET owner_user_id = $2,
    updated_at = NOW()
WHERE id = ANY(
  SELECT ai.control_record_id
  FROM assessment_items ai
  WHERE ai.assessment_id = $3
    AND ai.id = ANY($1::uuid[])
);

-- name: BulkAssignControlRecordReviewer :exec
UPDATE control_records
SET reviewer_user_id = $2,
    updated_at = NOW()
WHERE id = ANY(
  SELECT ai.control_record_id
  FROM assessment_items ai
  WHERE ai.assessment_id = $3
    AND ai.id = ANY($1::uuid[])
);
