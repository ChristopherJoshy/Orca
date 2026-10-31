-- +goose Up
CREATE TABLE stripe_reconciliation_outbox (
    id uuid PRIMARY KEY,
    provider_event_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    session_id text NOT NULL DEFAULT '',
    checkout_id uuid,
    business_id uuid REFERENCES businesses(id) ON DELETE SET NULL,
    reason text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    resolved_at timestamptz
);

CREATE INDEX stripe_reconciliation_pending
    ON stripe_reconciliation_outbox(created_at)
    WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS stripe_reconciliation_outbox;
