CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE assessment_status AS ENUM ('draft', 'active', 'completed', 'archived');
CREATE TYPE assessment_item_status AS ENUM (
  'not_started',
  'in_progress',
  'ready_for_review',
  'validated',
  'not_applicable',
  'blocked'
);
CREATE TYPE item_priority AS ENUM ('low', 'medium', 'high', 'critical');

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE frameworks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  label TEXT NOT NULL,
  version TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (slug, version)
);

CREATE TABLE framework_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  framework_id UUID NOT NULL REFERENCES frameworks(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  UNIQUE (framework_id, code)
);

CREATE TABLE framework_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  framework_id UUID NOT NULL REFERENCES frameworks(id) ON DELETE CASCADE,
  framework_group_id UUID NOT NULL REFERENCES framework_groups(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  asset_class TEXT NOT NULL,
  security_function TEXT NOT NULL,
  tags TEXT[] NOT NULL DEFAULT '{}',
  UNIQUE (framework_id, code)
);

CREATE TABLE assessments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  framework_id UUID NOT NULL REFERENCES frameworks(id),
  name TEXT NOT NULL,
  scope TEXT NOT NULL,
  start_date DATE NOT NULL,
  due_date DATE NOT NULL,
  status assessment_status NOT NULL DEFAULT 'active',
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE assessment_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  assessment_id UUID NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
  framework_item_id UUID NOT NULL REFERENCES framework_items(id),
  owner_user_id UUID REFERENCES users(id),
  reviewer_user_id UUID REFERENCES users(id),
  status assessment_item_status NOT NULL DEFAULT 'not_started',
  score INT CHECK (score BETWEEN 1 AND 5),
  priority item_priority NOT NULL DEFAULT 'medium',
  due_date DATE NOT NULL,
  notes TEXT,
  blocked_reason TEXT,
  validated_at TIMESTAMPTZ,
  validated_by UUID REFERENCES users(id),
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (assessment_id, framework_item_id)
);

CREATE TABLE evidence_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  assessment_item_id UUID NOT NULL REFERENCES assessment_items(id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  url TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE comments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  assessment_item_id UUID NOT NULL REFERENCES assessment_items(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  action TEXT NOT NULL,
  actor_user_id UUID NOT NULL REFERENCES users(id),
  payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_framework_groups_framework ON framework_groups(framework_id);
CREATE INDEX idx_framework_items_framework ON framework_items(framework_id);
CREATE INDEX idx_framework_items_group ON framework_items(framework_group_id);
CREATE INDEX idx_assessments_framework ON assessments(framework_id);
CREATE INDEX idx_assessment_items_assessment ON assessment_items(assessment_id);
CREATE INDEX idx_assessment_items_owner ON assessment_items(owner_user_id);
CREATE INDEX idx_assessment_items_reviewer ON assessment_items(reviewer_user_id);
CREATE INDEX idx_assessment_items_status ON assessment_items(status);
CREATE INDEX idx_assessment_items_due_date ON assessment_items(due_date);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id, created_at DESC);
