DROP INDEX IF EXISTS idx_audit_log_entity;
DROP INDEX IF EXISTS idx_assessment_items_due_date;
DROP INDEX IF EXISTS idx_assessment_items_status;
DROP INDEX IF EXISTS idx_assessment_items_reviewer;
DROP INDEX IF EXISTS idx_assessment_items_owner;
DROP INDEX IF EXISTS idx_assessment_items_assessment;
DROP INDEX IF EXISTS idx_assessments_framework;
DROP INDEX IF EXISTS idx_framework_items_group;
DROP INDEX IF EXISTS idx_framework_items_framework;
DROP INDEX IF EXISTS idx_framework_groups_framework;

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS evidence_links;
DROP TABLE IF EXISTS assessment_items;
DROP TABLE IF EXISTS assessments;
DROP TABLE IF EXISTS framework_items;
DROP TABLE IF EXISTS framework_groups;
DROP TABLE IF EXISTS frameworks;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS item_priority;
DROP TYPE IF EXISTS assessment_item_status;
DROP TYPE IF EXISTS assessment_status;
