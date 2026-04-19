ALTER TABLE users
ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE users
SET is_admin = role = 'admin'::user_role;

ALTER TABLE users
DROP COLUMN role;

DROP TYPE user_role;
