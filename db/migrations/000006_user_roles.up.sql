CREATE TYPE user_role AS ENUM ('admin', 'assessment_manager', 'editor', 'viewer');

ALTER TABLE users
ADD COLUMN role user_role;

UPDATE users
SET role = CASE
  WHEN is_admin THEN 'admin'::user_role
  ELSE 'editor'::user_role
END;

ALTER TABLE users
ALTER COLUMN role SET DEFAULT 'editor',
ALTER COLUMN role SET NOT NULL;

ALTER TABLE users
DROP COLUMN is_admin;
