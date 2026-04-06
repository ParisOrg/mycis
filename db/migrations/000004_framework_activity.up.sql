ALTER TABLE framework_groups
ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE framework_items
ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_framework_groups_framework_active
ON framework_groups(framework_id, is_active);

CREATE INDEX idx_framework_items_framework_active
ON framework_items(framework_id, is_active);
