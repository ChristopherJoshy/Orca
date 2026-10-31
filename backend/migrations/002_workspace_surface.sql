-- +goose Up
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS plan_id uuid;
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS amount numeric(20,2) NOT NULL DEFAULT 0;
ALTER TABLE quotations ADD CONSTRAINT quotations_plan_tenant_fk FOREIGN KEY (business_id, plan_id) REFERENCES plans(business_id, id);

CREATE TABLE tax_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id uuid NOT NULL REFERENCES businesses(id),
    name text NOT NULL,
    region text NOT NULL DEFAULT '',
    rate numeric(9,6) NOT NULL DEFAULT 0 CHECK (rate >= 0 AND rate <= 100),
    active boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (business_id, id)
);

ALTER TABLE payments ADD CONSTRAINT payments_business_id_unique UNIQUE (business_id, id);
ALTER TABLE invoices ADD CONSTRAINT invoices_subscription_tenant_fk FOREIGN KEY (business_id, subscription_id) REFERENCES subscriptions(business_id, id);
ALTER TABLE payment_allocations ADD CONSTRAINT payment_allocations_payment_tenant_fk FOREIGN KEY (business_id, payment_id) REFERENCES payments(business_id, id);
ALTER TABLE payment_allocations ADD CONSTRAINT payment_allocations_invoice_tenant_fk FOREIGN KEY (business_id, invoice_id) REFERENCES invoices(business_id, id);

-- +goose Down
ALTER TABLE payment_allocations DROP CONSTRAINT IF EXISTS payment_allocations_invoice_tenant_fk;
ALTER TABLE payment_allocations DROP CONSTRAINT IF EXISTS payment_allocations_payment_tenant_fk;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_subscription_tenant_fk;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_business_id_unique;
DROP TABLE IF EXISTS tax_rules;
ALTER TABLE quotations DROP CONSTRAINT IF EXISTS quotations_plan_tenant_fk;
ALTER TABLE quotations DROP COLUMN IF EXISTS amount;
ALTER TABLE quotations DROP COLUMN IF EXISTS plan_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS title;
