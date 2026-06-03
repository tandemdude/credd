-- +goose Up
CREATE TABLE secret (
    name TEXT PRIMARY KEY,
    encrypted_value TEXT NOT NULL
);

-- +goose Down
DROP TABLE secret;
