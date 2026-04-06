-- Step 1: Create assessment-scoped control_records table.
CREATE TABLE control_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  assessment_id UUID NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
  framework_item_id UUID NOT NULL REFERENCES framework_items(id) ON DELETE CASCADE,
  owner_user_id UUID REFERENCES users(id),
  reviewer_user_id UUID REFERENCES users(id),
  notes TEXT NOT NULL DEFAULT '',
  is_not_applicable BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (assessment_id, framework_item_id)
);

CREATE INDEX idx_control_records_owner ON control_records(owner_user_id);
CREATE INDEX idx_control_records_reviewer ON control_records(reviewer_user_id);

-- Step 2: Backfill one control_record per assessment item.
INSERT INTO control_records (
  assessment_id,
  framework_item_id,
  owner_user_id,
  reviewer_user_id,
  notes,
  is_not_applicable
)
SELECT
  ai.assessment_id,
  ai.framework_item_id,
  ai.owner_user_id,
  ai.reviewer_user_id,
  COALESCE(ai.notes, ''),
  ai.status = 'not_applicable'
FROM assessment_items ai;

-- Step 3: Add control_record_id column to assessment_items.
ALTER TABLE assessment_items ADD COLUMN control_record_id UUID REFERENCES control_records(id) ON DELETE CASCADE;

UPDATE assessment_items ai
SET control_record_id = cr.id
FROM control_records cr
WHERE cr.assessment_id = ai.assessment_id
  AND cr.framework_item_id = ai.framework_item_id;

ALTER TABLE assessment_items ALTER COLUMN control_record_id SET NOT NULL;

CREATE INDEX idx_assessment_items_control_record ON assessment_items(control_record_id);

-- Step 4: Re-parent evidence_links from assessment_item_id to control_record_id.
ALTER TABLE evidence_links ADD COLUMN control_record_id UUID REFERENCES control_records(id) ON DELETE CASCADE;

UPDATE evidence_links el
SET control_record_id = ai.control_record_id
FROM assessment_items ai
WHERE ai.id = el.assessment_item_id;

ALTER TABLE evidence_links ALTER COLUMN control_record_id SET NOT NULL;
ALTER TABLE evidence_links DROP COLUMN assessment_item_id;

CREATE INDEX idx_evidence_links_control_record ON evidence_links(control_record_id);

-- Step 5: Re-parent comments from assessment_item_id to control_record_id.
ALTER TABLE comments ADD COLUMN control_record_id UUID REFERENCES control_records(id) ON DELETE CASCADE;

UPDATE comments c
SET control_record_id = ai.control_record_id
FROM assessment_items ai
WHERE ai.id = c.assessment_item_id;

ALTER TABLE comments ALTER COLUMN control_record_id SET NOT NULL;
ALTER TABLE comments DROP COLUMN assessment_item_id;

CREATE INDEX idx_comments_control_record ON comments(control_record_id);

-- Step 6: Drop moved columns from assessment_items.
DROP INDEX IF EXISTS idx_assessment_items_owner;
DROP INDEX IF EXISTS idx_assessment_items_reviewer;

ALTER TABLE assessment_items DROP COLUMN owner_user_id;
ALTER TABLE assessment_items DROP COLUMN reviewer_user_id;
ALTER TABLE assessment_items DROP COLUMN notes;
