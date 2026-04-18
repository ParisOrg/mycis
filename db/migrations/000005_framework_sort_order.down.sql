DROP INDEX IF EXISTS idx_framework_items_framework_sort_order;
DROP INDEX IF EXISTS idx_framework_groups_framework_sort_order;

ALTER TABLE framework_items
DROP COLUMN IF EXISTS sort_order;

ALTER TABLE framework_groups
DROP COLUMN IF EXISTS sort_order;
