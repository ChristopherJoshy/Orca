-- +goose Up
ALTER TABLE quotation_templates
    ADD COLUMN plan_id uuid,
    ADD CONSTRAINT quotation_templates_plan_business_fk
        FOREIGN KEY (business_id, plan_id) REFERENCES plans(business_id, id);

-- +goose Down
ALTER TABLE quotation_templates
    DROP CONSTRAINT IF EXISTS quotation_templates_plan_business_fk,
    DROP COLUMN IF EXISTS plan_id;
