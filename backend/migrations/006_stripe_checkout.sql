-- +goose Up
CREATE TABLE stripe_checkouts (
    id uuid PRIMARY KEY,
    business_id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    amount numeric(20,2) NOT NULL CHECK (amount > 0),
    currency char(3) NOT NULL,
    idempotency_key text NOT NULL,
    provider_session_id text UNIQUE,
    checkout_url text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('Creating', 'Open', 'Processing', 'Completed', 'Expired', 'Failed', 'ReconciliationRequired')),
    expires_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    FOREIGN KEY (business_id, invoice_id) REFERENCES invoices(business_id, id),
    FOREIGN KEY (business_id, customer_id) REFERENCES customers(business_id, id),
    UNIQUE (business_id, idempotency_key)
);

CREATE UNIQUE INDEX stripe_one_active_checkout_per_invoice
    ON stripe_checkouts (business_id, invoice_id)
    WHERE status IN ('Creating', 'Open', 'Processing');
CREATE INDEX stripe_checkouts_invoice ON stripe_checkouts(business_id, invoice_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stripe_checkouts;
