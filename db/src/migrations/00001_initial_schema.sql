-- +goose Up
CREATE TABLE profile (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- A profile_var is a single NAME=value pairing within a profile. Secret values
-- are never stored here: a 'secret' var holds a reference (or a template of
-- references) that is resolved from an external secret manager at run time.
CREATE TABLE profile_var (
    profile_id INTEGER NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('plain', 'secret')),
    PRIMARY KEY (profile_id, name)
);

-- +goose Down
DROP TABLE profile_var;
DROP TABLE profile;
