-- Note: secret values are never stored in the database; they are always
-- resolved from an external secret manager (e.g. 1Password).

-- name: CreateProfile :execrows
INSERT INTO profile (name, description) VALUES (?, ?)
ON CONFLICT (name) DO NOTHING;

-- name: ListProfiles :many
SELECT name, description FROM profile ORDER BY name;

-- name: GetProfileID :one
SELECT id FROM profile WHERE name = ?;

-- name: GetProfile :one
SELECT id, description FROM profile WHERE name = ?;

-- name: UpdateProfileDescription :execrows
UPDATE profile SET description = ? WHERE name = ?;

-- name: DeleteProfile :execrows
DELETE FROM profile WHERE name = ?;

-- name: ListProfileVars :many
SELECT name, value, kind FROM profile_var WHERE profile_id = ? ORDER BY name;

-- name: UpsertProfileVar :exec
INSERT INTO profile_var (profile_id, name, value, kind) VALUES (?, ?, ?, ?)
ON CONFLICT (profile_id, name) DO UPDATE SET value = excluded.value, kind = excluded.kind;

-- name: DeleteProfileVar :execrows
DELETE FROM profile_var WHERE profile_id = ? AND name = ?;
