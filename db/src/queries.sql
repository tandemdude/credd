-- name: CreateSecret :exec
INSERT INTO secret (name, encrypted_value) VALUES (?, ?)
ON CONFLICT (name) DO UPDATE SET encrypted_value = excluded.encrypted_value;

-- name: ListSecrets :many
SELECT * FROM secret ORDER BY name;

-- name: GetSecret :one
SELECT * FROM secret WHERE name = ?;

-- name: DeleteSecret :execrows
DELETE FROM secret WHERE name = ?;

-- name: DeleteAllSecrets :exec
DELETE FROM secret;
