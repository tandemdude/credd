-- +goose Up
ALTER TABLE profile ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE profile DROP COLUMN description;
