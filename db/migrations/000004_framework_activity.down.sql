DROP INDEX IF EXISTS idx_framework_items_framework_active;
DROP INDEX IF EXISTS idx_framework_groups_framework_active;

ALTER TABLE framework_items
DROP COLUMN is_active;

ALTER TABLE framework_groups
DROP COLUMN is_active;
