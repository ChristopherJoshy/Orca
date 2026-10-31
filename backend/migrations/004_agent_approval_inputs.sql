-- +goose Up
ALTER TABLE approvals ADD COLUMN IF NOT EXISTS inputs jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE approvals DROP COLUMN IF EXISTS inputs;
