-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    email_normalized text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    email_verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE businesses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    currency char(3) NOT NULL DEFAULT 'INR' CHECK (currency = 'INR'),
    timezone text NOT NULL DEFAULT 'Asia/Kolkata',
    next_invoice_number bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    business_id uuid NOT NULL REFERENCES businesses(id),
    user_id uuid NOT NULL REFERENCES users(id),
    role text NOT NULL CHECK (role IN ('admin','staff','customer')),
    customer_id uuid,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, user_id)
);
CREATE INDEX memberships_user_active ON memberships(user_id, business_id) WHERE revoked_at IS NULL;

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    csrf_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);

CREATE TABLE auth_tokens (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_type text NOT NULL CHECK (token_type IN ('verify_email','password_reset')),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    user_id uuid REFERENCES users(id),
    name text NOT NULL,
    email text NOT NULL,
    company text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    billing_address jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, id)
);
CREATE UNIQUE INDEX customers_business_email ON customers(business_id, lower(email));

CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    name text NOT NULL,
    product_type text NOT NULL,
    sales_price numeric(20,6) NOT NULL CHECK (sales_price >= 0),
    cost_price numeric(20,6) NOT NULL DEFAULT 0 CHECK (cost_price >= 0),
    notes text NOT NULL DEFAULT '',
    variants jsonb NOT NULL DEFAULT '[]'::jsonb,
    tax_rate numeric(9,6) NOT NULL DEFAULT 0 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, id)
);
CREATE INDEX products_business ON products(business_id, name);

CREATE TABLE plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    name text NOT NULL,
    interval text NOT NULL CHECK (interval IN ('daily','weekly','monthly','yearly')),
    sales_price numeric(20,6) NOT NULL CHECK (sales_price >= 0),
    minimum_quantity integer NOT NULL DEFAULT 1 CHECK (minimum_quantity > 0),
    available_from date,
    available_until date,
    tax_rate numeric(9,6) NOT NULL DEFAULT 0 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    auto_close boolean NOT NULL DEFAULT false,
    allow_early_close boolean NOT NULL DEFAULT true,
    allow_pause boolean NOT NULL DEFAULT true,
    allow_renew boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, id),
    CHECK (available_until IS NULL OR available_from IS NULL OR available_until >= available_from)
);
CREATE INDEX plans_business ON plans(business_id, name);

CREATE TABLE discounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('fixed','percentage')),
    value numeric(20,6) NOT NULL CHECK (value >= 0),
    min_purchase numeric(20,6) NOT NULL DEFAULT 0,
    min_quantity integer NOT NULL DEFAULT 0,
    starts_at timestamptz,
    ends_at timestamptz,
    max_uses bigint,
    use_count bigint NOT NULL DEFAULT 0,
    scope text NOT NULL CHECK (scope IN ('product','subscription')),
    product_ids uuid[] NOT NULL DEFAULT '{}',
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (business_id, id)
);

CREATE TABLE quotation_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    name text NOT NULL,
    valid_days integer NOT NULL DEFAULT 30 CHECK (valid_days > 0),
    lines jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, id)
);

CREATE TABLE quotations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    customer_id uuid NOT NULL,
    number text NOT NULL,
    status text NOT NULL CHECK (status IN ('Draft','Quotation','Confirmed','Active','Closed')),
    valid_until date,
    lines jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, customer_id) REFERENCES customers(business_id, id),
    UNIQUE (business_id, id),
    UNIQUE (business_id, number)
);

CREATE TABLE subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    customer_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    discount_id uuid,
    number text NOT NULL,
    status text NOT NULL CHECK (status IN ('Draft','Quotation','Confirmed','Active','Closed')),
    start_date date NOT NULL,
    expiry_date date,
    billing_cursor timestamptz,
    payment_terms_days integer NOT NULL DEFAULT 14,
    lines jsonb NOT NULL,
    pause_intervals jsonb NOT NULL DEFAULT '[]'::jsonb,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, customer_id) REFERENCES customers(business_id, id),
    FOREIGN KEY (business_id, plan_id) REFERENCES plans(business_id, id),
    FOREIGN KEY (business_id, discount_id) REFERENCES discounts(business_id, id),
    UNIQUE (business_id, id),
    UNIQUE (business_id, number)
);
CREATE INDEX subscriptions_due ON subscriptions(business_id, billing_cursor) WHERE status = 'Active';

CREATE TABLE invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    subscription_id uuid,
    customer_id uuid NOT NULL,
    number text NOT NULL,
    period_start timestamptz NOT NULL,
    period_end timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('Draft','Confirmed','Paid','Cancelled')),
    currency char(3) NOT NULL DEFAULT 'INR',
    subtotal numeric(20,2) NOT NULL,
    discount_total numeric(20,2) NOT NULL,
    tax_total numeric(20,2) NOT NULL,
    total numeric(20,2) NOT NULL,
    due_at timestamptz NOT NULL,
    snapshot jsonb NOT NULL,
    calculation_hash char(64) NOT NULL,
    calculation_engine text NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, customer_id) REFERENCES customers(business_id, id),
    UNIQUE (business_id, id),
    UNIQUE (business_id, number),
    UNIQUE (business_id, subscription_id, period_start),
    CHECK (period_end > period_start)
);
CREATE INDEX invoices_receivable ON invoices(business_id, due_at) WHERE status = 'Confirmed';

CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    customer_id uuid NOT NULL,
    method text NOT NULL,
    amount numeric(20,2) NOT NULL CHECK (amount > 0),
    received_at timestamptz NOT NULL,
    note text NOT NULL DEFAULT '',
    provider text NOT NULL DEFAULT 'manual',
    provider_operation_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (business_id, customer_id) REFERENCES customers(business_id, id),
    UNIQUE (business_id, provider_operation_key)
);

CREATE TABLE payment_allocations (
    business_id uuid NOT NULL,
    payment_id uuid NOT NULL REFERENCES payments(id),
    invoice_id uuid NOT NULL REFERENCES invoices(id),
    amount numeric(20,2) NOT NULL CHECK (amount > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (business_id, payment_id, invoice_id)
);

CREATE TABLE journal_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    operation_key text NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    account text NOT NULL,
    debit numeric(20,2) NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit numeric(20,2) NOT NULL DEFAULT 0 CHECK (credit >= 0),
    currency char(3) NOT NULL DEFAULT 'INR',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, operation_key, account),
    CHECK ((debit = 0) <> (credit = 0))
);
CREATE INDEX journal_source ON journal_entries(business_id, source_type, source_id);

CREATE TABLE audit_events (
	 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    actor_user_id uuid REFERENCES users(id),
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_business_time ON audit_events(business_id, created_at DESC);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    event_type text NOT NULL,
    idempotency_key text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    UNIQUE (business_id, idempotency_key)
);
CREATE INDEX outbox_pending ON outbox_events(created_at) WHERE delivered_at IS NULL;

CREATE TABLE webhook_events (
    provider text NOT NULL,
    event_id text NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb NOT NULL,
    processed_at timestamptz,
    PRIMARY KEY (provider, event_id)
);

CREATE TABLE agent_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    user_id uuid NOT NULL REFERENCES users(id),
    conversation_id uuid NOT NULL,
    status text NOT NULL,
    message text NOT NULL,
    transcript jsonb NOT NULL DEFAULT '[]'::jsonb,
    outcome jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    actor_user_id uuid NOT NULL REFERENCES users(id),
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    inputs_hash char(64) NOT NULL,
    resource_version bigint NOT NULL DEFAULT 0,
    status text NOT NULL CHECK (status IN ('Pending','Approved','Rejected','Consumed','Expired')),
    expires_at timestamptz NOT NULL,
    decided_by uuid REFERENCES users(id),
    decided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX approvals_actor_pending ON approvals(business_id, actor_user_id, expires_at) WHERE status = 'Pending';

CREATE TABLE export_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    requested_by uuid NOT NULL REFERENCES users(id),
    status text NOT NULL CHECK (status IN ('Queued','Running','Complete','Failed','Expired')),
    filters jsonb NOT NULL DEFAULT '{}'::jsonb,
    snapshot_at timestamptz,
    manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
    archive_path text,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE oauth_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    user_id uuid NOT NULL REFERENCES users(id),
    client_id text NOT NULL,
    redirect_uri text NOT NULL,
    scopes text[] NOT NULL,
    token_hash bytea,
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
