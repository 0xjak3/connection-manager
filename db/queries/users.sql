-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password_hash), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetUserByID :one
SELECT id, email, password_hash, created_at, updated_at
FROM users
WHERE id = sqlc.arg(id);

-- name: ListUsers :many
SELECT id, email, password_hash, created_at, updated_at
FROM users
ORDER BY created_at, id;

-- name: UpdateUser :execrows
UPDATE users
SET email = sqlc.arg(email),
    password_hash = sqlc.arg(password_hash),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = sqlc.arg(id);
