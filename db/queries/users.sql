-- name: CreateUser :one
INSERT INTO users (
  name,
  email,
  password_hash,
  role,
  must_change_password
) VALUES (
  $1,
  lower($2),
  $3,
  $4,
  $5
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = lower($1);

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY CASE role
  WHEN 'admin' THEN 0
  WHEN 'assessment_manager' THEN 1
  WHEN 'editor' THEN 2
  ELSE 3
END,
name ASC;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2,
    must_change_password = FALSE,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    role = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPasswordReset :exec
UPDATE users
SET password_hash = $2,
    must_change_password = TRUE,
    updated_at = NOW()
WHERE id = $1;
