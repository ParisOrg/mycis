-- Reverse of 000003_control_records.up.sql

-- Step 1: Re-add columns to assessment_items.
ALTER TABLE assessment_items ADD COLUMN owner_user_id UUID REFERENCES users(id);
ALTER TABLE assessment_items ADD COLUMN reviewer_user_id UUID REFERENCES users(id);
ALTER TABLE assessment_items ADD COLUMN notes TEXT;

CREATE INDEX idx_assessment_items_owner ON assessment_items(owner_user_id);
CREATE INDEX idx_assessment_items_reviewer ON assessment_items(reviewer_user_id);

-- Step 2: Backfill from control_records.
UPDATE assessment_items ai
SET owner_user_id = cr.owner_user_id,
    reviewer_user_id = cr.reviewer_user_id,
    notes = NULLIF(cr.notes, '')
FROM control_records cr
WHERE cr.id = ai.control_record_id;

-- Step 3: Re-parent comments back to assessment_item_id.
ALTER TABLE comments ADD COLUMN assessment_item_id UUID;

UPDATE comments c
SET assessment_item_id = ai.id
FROM assessment_items ai
WHERE ai.control_record_id = c.control_record_id;

ALTER TABLE comments ALTER COLUMN assessment_item_id SET NOT NULL;
ALTER TABLE comments ADD CONSTRAINT comments_assessment_item_id_fkey
  FOREIGN KEY (assessment_item_id) REFERENCES assessment_items(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_comments_control_record;
ALTER TABLE comments DROP COLUMN control_record_id;

-- Step 4: Re-parent evidence_links back to assessment_item_id.
ALTER TABLE evidence_links ADD COLUMN assessment_item_id UUID;

UPDATE evidence_links el
SET assessment_item_id = ai.id
FROM assessment_items ai
WHERE ai.control_record_id = el.control_record_id;

ALTER TABLE evidence_links ALTER COLUMN assessment_item_id SET NOT NULL;
ALTER TABLE evidence_links ADD CONSTRAINT evidence_links_assessment_item_id_fkey
  FOREIGN KEY (assessment_item_id) REFERENCES assessment_items(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_evidence_links_control_record;
ALTER TABLE evidence_links DROP COLUMN control_record_id;

-- Step 5: Drop control_record_id from assessment_items.
DROP INDEX IF EXISTS idx_assessment_items_control_record;
ALTER TABLE assessment_items DROP COLUMN control_record_id;

-- Step 6: Drop control_records table.
DROP INDEX IF EXISTS idx_control_records_reviewer;
DROP INDEX IF EXISTS idx_control_records_owner;
DROP TABLE IF EXISTS control_records;
