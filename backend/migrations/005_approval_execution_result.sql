-- +goose Up
ALTER TABLE approvals ADD COLUMN IF NOT EXISTS execution_result jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE approvals DROP COLUMN IF EXISTS execution_result;
