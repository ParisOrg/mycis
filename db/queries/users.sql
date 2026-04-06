-- name: CreateUser :one
INSERT INTO users (
  name,
  email,
  password_hash,
  is_admin,
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
ORDER BY is_admin DESC, name ASC;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2,
    must_change_password = FALSE,
    updated_at = NOW()
WHERE id = $1;
